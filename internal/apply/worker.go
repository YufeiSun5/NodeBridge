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
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

const maxStatementPlaceholders = 60000

type SQLWorker struct {
	DB    *sql.DB
	Clock func() time.Time
}

func NewSQLWorker(db *sql.DB) *SQLWorker {
	return &SQLWorker{DB: db, Clock: time.Now}
}

func (w *SQLWorker) ApplyBatch(ctx context.Context, events []mapper.MappedEvent) (BatchResult, error) {
	if w.DB == nil {
		return BatchResult{}, errors.New("mysql db is nil")
	}
	if w.Clock == nil {
		w.Clock = time.Now
	}
	for _, evt := range events {
		if err := validateMappedEvent(evt); err != nil {
			return BatchResult{}, err
		}
	}
	if len(events) == 0 {
		return BatchResult{}, nil
	}

	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return BatchResult{}, fmt.Errorf("begin batch apply tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	existing, err := existingApplyLogs(ctx, tx, events)
	if err != nil {
		return BatchResult{}, err
	}
	if canApplyAppendOnlyUnordered(events, existing) {
		results, err := applyAppendOnlyUnorderedBatch(ctx, tx, events, existing, w.Clock())
		if err != nil {
			return BatchResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return BatchResult{}, fmt.Errorf("commit append_only batch apply tx: %w", err)
		}
		committed = true
		return BatchResult{Results: results}, nil
	}

	results := make([]Result, 0, len(events))
	for index := 0; index < len(events); {
		evt := events[index]
		result := Result{
			EventID:     evt.Event.EventID,
			SourceTable: evt.SourceTable,
			TargetTable: evt.TargetTable,
		}
		if existing[evt.Event.EventID] {
			result.AlreadyApplied = true
			results = append(results, result)
			index++
			continue
		}
		savepoint := fmt.Sprintf("nb_apply_%d", index)
		if err := execTx(ctx, tx, "SAVEPOINT "+savepoint); err != nil {
			return BatchResult{}, err
		}
		group := []mapper.MappedEvent{evt}
		if isAppendOnlyInsert(evt) {
			group = appendOnlySegment(events, existing, index)
		} else if isCompactUpdate(evt) {
			group = compactUpdateGroup(events, existing, index)
		}
		groupResults, err := applyGroupInTx(ctx, tx, group, existing, w.Clock())
		if err != nil {
			if rollbackErr := rollbackSavepoint(ctx, tx, savepoint); rollbackErr != nil {
				return BatchResult{}, fmt.Errorf("%w; rollback savepoint failed: %v", err, rollbackErr)
			}
			if len(results) > 0 {
				if commitErr := tx.Commit(); commitErr != nil {
					return BatchResult{}, fmt.Errorf("%w; commit applied prefix failed: %v", err, commitErr)
				}
				committed = true
			}
			return BatchResult{Results: results}, err
		}
		if err := execTx(ctx, tx, "RELEASE SAVEPOINT "+savepoint); err != nil {
			return BatchResult{}, err
		}
		for _, applied := range groupResults {
			existing[applied.EventID] = true
			results = append(results, applied)
		}
		index += len(group)
	}
	if err := tx.Commit(); err != nil {
		return BatchResult{}, fmt.Errorf("commit batch apply tx: %w", err)
	}
	committed = true
	return BatchResult{Results: results}, nil
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

	result, err := w.applyOne(ctx, tx, mapped)
	if err != nil {
		return Result{}, err
	}
	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit apply tx: %w", err)
	}
	return result, nil
}

func (w *SQLWorker) applyOne(ctx context.Context, tx *sql.Tx, mapped mapper.MappedEvent) (Result, error) {
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
		return result, nil
	}

	if err := applyMappedInTx(ctx, tx, mapped, w.Clock()); err != nil {
		return Result{}, err
	}
	return result, nil
}

func applyMappedInTx(ctx context.Context, tx *sql.Tx, mapped mapper.MappedEvent, now time.Time) error {
	if mapped.SyncMode == rules.SyncModeAppendOnly && mapped.Event.EventType != event.TypeInsert {
		return fmt.Errorf("append_only table rejects %s event %s", mapped.Event.EventType, mapped.Event.EventID)
	}
	var err error
	switch mapped.Event.EventType {
	case event.TypeInsert:
		err = applyInsert(ctx, tx, mapped)
	case event.TypeUpdate:
		err = applyUpdate(ctx, tx, mapped)
	case event.TypeDelete:
		err = applySoftDelete(ctx, tx, mapped, now)
	default:
		err = fmt.Errorf("unsupported event type %q", mapped.Event.EventType)
	}
	if err != nil {
		return err
	}
	if err := insertApplyLog(ctx, tx, mapped, now); err != nil {
		return err
	}
	return nil
}

func applyGroupInTx(ctx context.Context, tx *sql.Tx, events []mapper.MappedEvent, existing map[string]bool, now time.Time) ([]Result, error) {
	if len(events) == 0 {
		return nil, nil
	}
	if isAppendOnlyInsert(events[0]) {
		return applyAppendOnlyUnorderedBatch(ctx, tx, events, existing, now)
	}
	if isCompactUpdate(events[0]) && len(events) > 1 {
		return applyCompactUpdateGroup(ctx, tx, events, now)
	}
	results := make([]Result, 0, len(events))
	for _, evt := range events {
		if err := applyMappedInTx(ctx, tx, evt, now); err != nil {
			return nil, err
		}
		results = append(results, Result{
			EventID:     evt.Event.EventID,
			SourceTable: evt.SourceTable,
			TargetTable: evt.TargetTable,
		})
	}
	return results, nil
}

func alreadyApplied(ctx context.Context, tx *sql.Tx, eventID string) (bool, error) {
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(1) FROM sync_apply_log WHERE event_id = ?", eventID).Scan(&count); err != nil {
		return false, fmt.Errorf("query apply log: %w", err)
	}
	return count > 0, nil
}

func existingApplyLogs(ctx context.Context, tx *sql.Tx, events []mapper.MappedEvent) (map[string]bool, error) {
	existing := make(map[string]bool, len(events))
	ids := make([]string, 0, len(events))
	for _, evt := range events {
		if evt.Event.EventID == "" || existing[evt.Event.EventID] {
			continue
		}
		existing[evt.Event.EventID] = false
		ids = append(ids, evt.Event.EventID)
	}
	if len(ids) == 0 {
		return existing, nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx, "SELECT event_id FROM sync_apply_log WHERE event_id IN ("+strings.Join(placeholders, ", ")+")", args...)
	if err != nil {
		return nil, fmt.Errorf("query batch apply log: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var eventID string
		if err := rows.Scan(&eventID); err != nil {
			return nil, fmt.Errorf("scan batch apply log: %w", err)
		}
		existing[eventID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate batch apply log: %w", err)
	}
	return existing, nil
}

func execTx(ctx context.Context, tx *sql.Tx, query string) error {
	if _, err := tx.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("exec tx statement %q: %w", query, err)
	}
	return nil
}

func rollbackSavepoint(ctx context.Context, tx *sql.Tx, savepoint string) error {
	if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+savepoint); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+savepoint)
	return err
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

func applyAppendOnlyInsertBatch(ctx context.Context, tx *sql.Tx, events []mapper.MappedEvent, now time.Time) error {
	if len(events) == 0 {
		return nil
	}
	columns := sortedKeys(events[0].TargetAfter)
	if len(columns) == 0 {
		return errors.New("append_only insert event after image is empty")
	}
	maxRows := maxRowsPerStatement(len(columns))
	for start := 0; start < len(events); start += maxRows {
		end := start + maxRows
		if end > len(events) {
			end = len(events)
		}
		if err := applyAppendOnlyInsertChunk(ctx, tx, events[start:end], columns); err != nil {
			return err
		}
	}
	if err := insertApplyLogs(ctx, tx, events, now); err != nil {
		return err
	}
	return nil
}

func applyAppendOnlyInsertChunk(ctx context.Context, tx *sql.Tx, events []mapper.MappedEvent, columns []string) error {
	rowPlaceholders := "(" + strings.TrimRight(strings.Repeat("?, ", len(columns)), ", ") + ")"
	valueParts := make([]string, 0, len(events))
	args := make([]any, 0, len(events)*len(columns))
	for _, evt := range events {
		if !isAppendOnlyInsert(evt) {
			return fmt.Errorf("append_only batch rejects %s event %s", evt.Event.EventType, evt.Event.EventID)
		}
		if !sameStringSlice(columns, sortedKeys(evt.TargetAfter)) {
			return errors.New("append_only batch requires identical target columns")
		}
		valueParts = append(valueParts, rowPlaceholders)
		for _, column := range columns {
			args = append(args, evt.TargetAfter[column])
		}
	}

	query := fmt.Sprintf(
		"INSERT IGNORE INTO %s (%s) VALUES %s",
		qualifiedTable(events[0].TargetDatabase, events[0].TargetTable),
		quoteJoin(columns),
		strings.Join(valueParts, ", "),
	)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("apply append_only insert batch: %w", err)
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

func insertApplyLogs(ctx context.Context, tx *sql.Tx, events []mapper.MappedEvent, now time.Time) error {
	if len(events) == 0 {
		return nil
	}
	maxRows := maxRowsPerStatement(11)
	for start := 0; start < len(events); start += maxRows {
		end := start + maxRows
		if end > len(events) {
			end = len(events)
		}
		if err := insertApplyLogChunk(ctx, tx, events[start:end], now); err != nil {
			return err
		}
	}
	return nil
}

func insertApplyLogChunk(ctx context.Context, tx *sql.Tx, events []mapper.MappedEvent, now time.Time) error {
	valueParts := make([]string, 0, len(events))
	args := make([]any, 0, len(events)*11)
	for _, mapped := range events {
		pkValue, err := json.Marshal(mapped.TargetPrimaryKey)
		if err != nil {
			return fmt.Errorf("marshal primary key: %w", err)
		}
		valueParts = append(valueParts, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
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
	}

	query := `INSERT INTO sync_apply_log (
event_id, origin_node_id, source_node_id, target_node_id,
database_name, table_name, target_database_name, target_table_name,
pk_value, op_type, applied_at
) VALUES ` + strings.Join(valueParts, ", ")
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert apply logs: %w", err)
	}
	return nil
}

func maxRowsPerStatement(placeholdersPerRow int) int {
	if placeholdersPerRow <= 0 {
		return 1
	}
	rows := maxStatementPlaceholders / placeholdersPerRow
	if rows < 1 {
		return 1
	}
	return rows
}

func appendOnlySegment(events []mapper.MappedEvent, existing map[string]bool, start int) []mapper.MappedEvent {
	first := events[start]
	group := []mapper.MappedEvent{first}
	for i := start + 1; i < len(events); i++ {
		next := events[i]
		if existing[next.Event.EventID] || !isAppendOnlyInsert(next) {
			break
		}
		group = append(group, next)
	}
	return group
}

func isAppendOnlyInsert(mapped mapper.MappedEvent) bool {
	return mapped.SyncMode == rules.SyncModeAppendOnly && mapped.Event.EventType == event.TypeInsert
}

func isCompactUpdate(mapped mapper.MappedEvent) bool {
	return mapped.SyncMode == rules.SyncModeCRUDCompact && mapped.Event.EventType == event.TypeUpdate
}

func compactUpdateGroup(events []mapper.MappedEvent, existing map[string]bool, start int) []mapper.MappedEvent {
	first := events[start]
	group := []mapper.MappedEvent{first}
	firstKey := compactPrimaryKey(first)
	for i := start + 1; i < len(events); i++ {
		next := events[i]
		if existing[next.Event.EventID] || !isCompactUpdate(next) {
			break
		}
		if next.TargetDatabase != first.TargetDatabase || next.TargetTable != first.TargetTable {
			break
		}
		if compactPrimaryKey(next) != firstKey {
			break
		}
		group = append(group, next)
	}
	return group
}

func applyCompactUpdateGroup(ctx context.Context, tx *sql.Tx, events []mapper.MappedEvent, now time.Time) ([]Result, error) {
	if len(events) == 0 {
		return nil, nil
	}
	last := events[len(events)-1]
	if err := applyUpdate(ctx, tx, last); err != nil {
		return nil, err
	}
	if err := insertApplyLogs(ctx, tx, events, now); err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(events))
	for _, evt := range events {
		results = append(results, Result{
			EventID:     evt.Event.EventID,
			SourceTable: evt.SourceTable,
			TargetTable: evt.TargetTable,
		})
	}
	return results, nil
}

func compactPrimaryKey(mapped mapper.MappedEvent) string {
	pk, err := json.Marshal(mapped.TargetPrimaryKey)
	if err != nil {
		return ""
	}
	return string(pk)
}

func canApplyAppendOnlyUnordered(events []mapper.MappedEvent, existing map[string]bool) bool {
	hasFresh := false
	for _, evt := range events {
		if existing[evt.Event.EventID] {
			continue
		}
		if !isAppendOnlyInsert(evt) {
			return false
		}
		hasFresh = true
	}
	return hasFresh
}

func applyAppendOnlyUnorderedBatch(ctx context.Context, tx *sql.Tx, events []mapper.MappedEvent, existing map[string]bool, now time.Time) ([]Result, error) {
	results := make([]Result, 0, len(events))
	groups := make(map[string][]mapper.MappedEvent)
	groupOrder := make([]string, 0)
	for _, evt := range events {
		result := Result{
			EventID:     evt.Event.EventID,
			SourceTable: evt.SourceTable,
			TargetTable: evt.TargetTable,
		}
		if existing[evt.Event.EventID] {
			result.AlreadyApplied = true
			results = append(results, result)
			continue
		}
		key := appendOnlyGroupKey(evt)
		if _, ok := groups[key]; !ok {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], evt)
		results = append(results, result)
	}
	for _, key := range groupOrder {
		if err := applyAppendOnlyInsertBatch(ctx, tx, groups[key], now); err != nil {
			return results, err
		}
	}
	return results, nil
}

func appendOnlyGroupKey(mapped mapper.MappedEvent) string {
	return mapped.TargetDatabase + "." + mapped.TargetTable + "|" + strings.Join(sortedKeys(mapped.TargetAfter), ",")
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

func sameStringSlice(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
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
