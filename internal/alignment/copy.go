package alignment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	hashpkg "hash"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type CopyResult struct {
	PlanID string `json:"plan_id"`
	Rows   int64  `json:"rows"`
	Digest string `json:"digest"`
}

// CopySnapshot is an offline copy primitive, not a CDC cutover operation.
// Callers own endpoint access and must fence application/CDC writers separately.
// It is deliberately not exposed by runtime, Wails or MCP before that fencing exists.
func CopySnapshot(ctx context.Context, source, target *sql.DB, plan Plan, rule rules.SyncRule, confirm bool) (CopyResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	result := CopyResult{PlanID: plan.ID}
	if err := plan.Validate(rule, time.Now(), confirm); err != nil {
		return result, err
	}
	if rule.Enable {
		return result, errors.New("alignment_requires_disabled_rule")
	}
	if source == nil || target == nil {
		return result, errors.New("alignment_endpoint_required")
	}
	src, releaseSource, err := beginSnapshotTx(ctx, source)
	if err != nil {
		return result, err
	}
	defer releaseSource()
	dst, releaseTarget, err := beginSnapshotTx(ctx, target)
	if err != nil {
		return result, err
	}
	defer releaseTarget()
	// Locking scans cover existing rows and insertion gaps until target commit.
	sourceCount, err := lockSnapshotTable(ctx, src, plan.Source.Schema)
	if err != nil {
		return result, err
	}
	targetCount, err := lockSnapshotTable(ctx, dst, plan.Target.Schema)
	if err != nil {
		return result, err
	}
	if targetCount != 0 {
		return result, errors.New("alignment_target_no_longer_empty")
	}
	if (sourceCount != 0) != plan.Source.HasRows {
		return result, errors.New("alignment_source_emptiness_changed")
	}
	for _, endpoint := range []struct {
		tx     *sql.Tx
		schema rulecheck.Schema
	}{{src, plan.Source.Schema}, {dst, plan.Target.Schema}} {
		actual, err := rulecheck.ReadSchema(ctx, endpoint.tx, endpoint.schema.Database, endpoint.schema.Table)
		if err != nil {
			return result, err
		}
		if hash(actual) != hash(endpoint.schema) {
			return result, errors.New("alignment_schema_changed")
		}
	}
	if err := rejectTargetSideEffects(ctx, dst, plan.Target.Schema); err != nil {
		return result, err
	}
	sourceColumns, targetColumns, targetOrder := copyColumns(plan)
	sourceHash := sha256.New()
	insert := "INSERT INTO " + qualified(plan.Target.Schema) + " (" + quoteColumns(targetColumns) + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(targetColumns)), ",") + ")"
	stmt, err := dst.PrepareContext(ctx, insert)
	if err != nil {
		return result, err
	}
	defer stmt.Close()
	count, err := scanSnapshot(ctx, src, plan.Source.Schema, sourceColumns, plan.Source.Schema.PrimaryKeys, func(values [][]byte) error {
		args := make([]any, len(values))
		for i, v := range values {
			if v != nil {
				args[i] = v
			}
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return err
		}
		writeRowDigest(sourceHash, values)
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("alignment_copy_failed: %w", err)
	}
	if count != sourceCount {
		return result, errors.New("alignment_source_count_changed")
	}
	targetHash := sha256.New()
	verified, err := scanSnapshot(ctx, dst, plan.Target.Schema, targetColumns, targetOrder, func(values [][]byte) error {
		writeRowDigest(targetHash, values)
		return nil
	})
	if err != nil {
		return result, err
	}
	if verified != count || !bytes.Equal(sourceHash.Sum(nil), targetHash.Sum(nil)) {
		return result, errors.New("alignment_copy_verification_failed")
	}
	if err := dst.Commit(); err != nil {
		return result, fmt.Errorf("alignment_commit_unknown: inspect target before replanning: %w", err)
	}
	result.Rows, result.Digest = count, hex.EncodeToString(sourceHash.Sum(nil))
	return result, nil
}

func beginSnapshotTx(ctx context.Context, db *sql.DB) (*sql.Tx, func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	var tx *sql.Tx
	release := func() {
		if tx != nil {
			_ = tx.Rollback()
		}
		// These infrequent maintenance connections must not leak session settings
		// into the application's pool, even if a reset query would fail.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()
	}
	if _, err := conn.ExecContext(ctx, "SET SESSION time_zone = '+00:00'"); err != nil {
		release()
		return nil, nil, err
	}
	tx, err = conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		release()
		return nil, nil, err
	}
	return tx, release, nil
}

func lockSnapshotTable(ctx context.Context, tx *sql.Tx, s rulecheck.Schema) (int64, error) {
	rows, err := tx.QueryContext(ctx, "SELECT "+quoteColumns(s.PrimaryKeys)+" FROM "+qualified(s)+" FORCE INDEX (PRIMARY) ORDER BY "+quoteColumns(s.PrimaryKeys)+" FOR UPDATE")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var count int64
	for rows.Next() {
		count++
	}
	return count, rows.Err()
}

func rejectTargetSideEffects(ctx context.Context, tx *sql.Tx, s rulecheck.Schema) error {
	if err := rulecheck.RequireNoTriggers(ctx, tx, s.Database, s.Table); err != nil {
		return fmt.Errorf("alignment_trigger_check: %w", err)
	}
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND REFERENCED_TABLE_NAME IS NOT NULL", s.Database, s.Table).Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return errors.New("alignment_target_foreign_keys_unsupported")
	}
	return nil
}

func copyColumns(plan Plan) (source, target, order []string) {
	for _, p := range plan.Columns {
		source = append(source, p.Source)
		target = append(target, p.Target)
	}
	for _, key := range plan.Source.Schema.PrimaryKeys {
		for _, p := range plan.Columns {
			if p.Source == key {
				order = append(order, p.Target)
			}
		}
	}
	return
}

func quoteColumns(columns []string) string {
	quoted := make([]string, len(columns))
	for i, c := range columns {
		quoted[i] = "`" + c + "`"
	}
	return strings.Join(quoted, ",")
}

func scanSnapshot(ctx context.Context, tx *sql.Tx, s rulecheck.Schema, columns, order []string, consume func([][]byte) error) (int64, error) {
	casts := make([]string, len(columns))
	for i, c := range columns {
		casts[i] = "CAST(`" + c + "` AS BINARY)"
	}
	rows, err := tx.QueryContext(ctx, "SELECT "+strings.Join(casts, ",")+" FROM "+qualified(s)+" ORDER BY "+quoteColumns(order))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var count int64
	for rows.Next() {
		values, scan := make([][]byte, len(columns)), make([]any, len(columns))
		for i := range scan {
			scan[i] = &values[i]
		}
		if err := rows.Scan(scan...); err != nil {
			return count, err
		}
		if err := consume(values); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func writeRowDigest(h hashpkg.Hash, values [][]byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(values)))
	_, _ = h.Write(length[:])
	for _, value := range values {
		if value == nil {
			_, _ = h.Write([]byte{0})
			continue
		}
		_, _ = h.Write([]byte{1})
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = h.Write(length[:])
		_, _ = h.Write(value)
	}
}
