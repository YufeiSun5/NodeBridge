package apply

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
)

type SQLWorker struct {
	DB    *sql.DB
	Clock func() time.Time
}

func NewSQLWorker(db *sql.DB) *SQLWorker {
	return &SQLWorker{DB: db, Clock: time.Now}
}

func (w *SQLWorker) Apply(ctx context.Context, mapped mapper.MappedEvent) (Result, error) {
	if w.DB == nil {
		return Result{}, errors.New("mysql db is nil")
	}
	if w.Clock == nil {
		w.Clock = time.Now
	}
	if err := validateMappedEvent(mapped); err != nil {
		return Result{}, err
	}

	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("begin apply tx: %w", err)
	}
	defer tx.Rollback()

	applied, err := alreadyApplied(ctx, tx, mapped.Event.EventID)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		EventID:     mapped.Event.EventID,
		SourceTable: mapped.SourceTable,
		TargetTable: mapped.TargetTable,
	}
	if applied {
		result.AlreadyApplied = true
		if err := tx.Commit(); err != nil {
			return Result{}, fmt.Errorf("commit idempotent apply: %w", err)
		}
		return result, nil
	}

	switch mapped.Event.EventType {
	case event.TypeInsert:
		err = applyInsert(ctx, tx, mapped)
	case event.TypeUpdate:
		err = applyUpdate(ctx, tx, mapped)
	case event.TypeDelete:
		err = applySoftDelete(ctx, tx, mapped, w.Clock())
	default:
		err = fmt.Errorf("unsupported event type %q", mapped.Event.EventType)
	}
	if err != nil {
		return Result{}, err
	}

	if err := insertApplyLog(ctx, tx, mapped, w.Clock()); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit apply tx: %w", err)
	}
	return result, nil
}

func alreadyApplied(ctx context.Context, tx *sql.Tx, eventID string) (bool, error) {
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(1) FROM sync_apply_log WHERE event_id = ?", eventID).Scan(&count); err != nil {
		return false, fmt.Errorf("query apply log: %w", err)
	}
	return count > 0, nil
}

func applyInsert(ctx context.Context, tx *sql.Tx, mapped mapper.MappedEvent) error {
	if len(mapped.TargetAfter) == 0 {
		return errors.New("insert event after image is empty")
	}
	columns := sortedKeys(mapped.TargetAfter)
	assignments := make([]string, 0, len(columns))
	placeholders := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns))
	for _, column := range columns {
		assignments = append(assignments, quoteIdentifier(column)+" = VALUES("+quoteIdentifier(column)+")")
		placeholders = append(placeholders, "?")
		args = append(args, mapped.TargetAfter[column])
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		qualifiedTable(mapped.TargetDatabase, mapped.TargetTable),
		quoteJoin(columns),
		strings.Join(placeholders, ", "),
		strings.Join(assignments, ", "),
	)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("apply insert: %w", err)
	}
	return nil
}

func applyUpdate(ctx context.Context, tx *sql.Tx, mapped mapper.MappedEvent) error {
	if len(mapped.TargetAfter) == 0 {
		return errors.New("update event after image is empty")
	}
	setColumns := nonPrimaryColumns(mapped.TargetAfter, mapped.TargetPrimaryKey)
	if len(setColumns) == 0 {
		return errors.New("update event has no non-primary columns")
	}

	setParts := make([]string, 0, len(setColumns))
	args := make([]any, 0, len(setColumns)+len(mapped.TargetPrimaryKey))
	for _, column := range setColumns {
		setParts = append(setParts, quoteIdentifier(column)+" = ?")
		args = append(args, mapped.TargetAfter[column])
	}
	where, whereArgs, err := wherePrimaryKey(mapped.TargetPrimaryKey)
	if err != nil {
		return err
	}
	args = append(args, whereArgs...)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s", qualifiedTable(mapped.TargetDatabase, mapped.TargetTable), strings.Join(setParts, ", "), where)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("apply update: %w", err)
	}
	return nil
}

func applySoftDelete(ctx context.Context, tx *sql.Tx, mapped mapper.MappedEvent, now time.Time) error {
	where, whereArgs, err := wherePrimaryKey(mapped.TargetPrimaryKey)
	if err != nil {
		return err
	}
	args := []any{1, now, mapped.Event.OriginNodeID, mapped.Event.OriginNodeID, mapped.Event.EventID}
	args = append(args, whereArgs...)

	query := fmt.Sprintf(
		"UPDATE %s SET `is_deleted` = ?, `deleted_at` = ?, `deleted_by_node` = ?, `updated_by_node` = ?, `last_event_id` = ? WHERE %s",
		qualifiedTable(mapped.TargetDatabase, mapped.TargetTable),
		where,
	)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("apply soft delete: %w", err)
	}
	return nil
}

func insertApplyLog(ctx context.Context, tx *sql.Tx, mapped mapper.MappedEvent, now time.Time) error {
	pkValue, err := json.Marshal(mapped.TargetPrimaryKey)
	if err != nil {
		return fmt.Errorf("marshal primary key: %w", err)
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO sync_apply_log (
event_id, origin_node_id, source_node_id, target_node_id,
database_name, table_name, target_database_name, target_table_name,
pk_value, op_type, applied_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mapped.Event.EventID,
		mapped.Event.OriginNodeID,
		mapped.Event.SourceNodeID,
		mapped.Event.TargetNodeID,
		mapped.SourceDatabase,
		mapped.SourceTable,
		mapped.TargetDatabase,
		mapped.TargetTable,
		string(pkValue),
		mapped.Event.EventType,
		now,
	)
	if err != nil {
		return fmt.Errorf("insert apply log: %w", err)
	}
	return nil
}

func validateMappedEvent(mapped mapper.MappedEvent) error {
	if mapped.Event.EventID == "" {
		return errors.New("event_id is required")
	}
	for _, value := range []string{mapped.TargetDatabase, mapped.TargetTable} {
		if err := mapper.ValidateIdentifier(value); err != nil {
			return err
		}
	}
	for _, values := range []map[string]any{mapped.TargetPrimaryKey, mapped.TargetBefore, mapped.TargetAfter} {
		for column := range values {
			if err := mapper.ValidateIdentifier(column); err != nil {
				return err
			}
		}
	}
	return nil
}

func wherePrimaryKey(primaryKey map[string]any) (string, []any, error) {
	if len(primaryKey) == 0 {
		return "", nil, errors.New("primary key is required")
	}
	columns := sortedKeys(primaryKey)
	parts := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns))
	for _, column := range columns {
		parts = append(parts, quoteIdentifier(column)+" = ?")
		args = append(args, primaryKey[column])
	}
	return strings.Join(parts, " AND "), args, nil
}

func nonPrimaryColumns(values, primaryKey map[string]any) []string {
	columns := make([]string, 0, len(values))
	for column := range values {
		if _, ok := primaryKey[column]; ok {
			continue
		}
		columns = append(columns, column)
	}
	sort.Strings(columns)
	return columns
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func quoteJoin(columns []string) string {
	quoted := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted = append(quoted, quoteIdentifier(column))
	}
	return strings.Join(quoted, ", ")
}

func qualifiedTable(database, table string) string {
	return quoteIdentifier(database) + "." + quoteIdentifier(table)
}

func quoteIdentifier(value string) string {
	return "`" + value + "`"
}
