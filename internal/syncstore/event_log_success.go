package syncstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// MarkEventLogsSucceeded preserves the payload committed before publisher confirmation.
func (s *Store) MarkEventLogsSucceeded(ctx context.Context, eventIDs []string, appliedAt time.Time) error {
	if s.DB == nil {
		return fmt.Errorf("sync store db is required")
	}
	ids := make([]string, 0, len(eventIDs))
	seen := make(map[string]bool, len(eventIDs))
	for _, id := range eventIDs {
		if id == "" {
			return fmt.Errorf("event_id is required")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if appliedAt.IsZero() {
		appliedAt = s.now()
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin event log success tx: %w", err)
	}
	defer tx.Rollback()
	if err := markEventLogsSucceededTx(ctx, tx, ids, appliedAt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit event log success tx: %w", err)
	}
	return nil
}

func markEventLogsSucceededTx(ctx context.Context, tx *sql.Tx, ids []string, appliedAt time.Time) error {
	for start := 0; start < len(ids); start += maxEventLogBatchRows {
		batch := ids[start:min(start+maxEventLogBatchRows, len(ids))]
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		args := []any{StatusSuccess, appliedAt}
		for _, id := range batch {
			args = append(args, id)
		}
		// A duplicate INSERT can lock the PRIMARY supremum and stall unrelated log inserts.
		result, err := tx.ExecContext(ctx, "UPDATE sync_event_log SET status=?, applied_at=?, error_message=NULL WHERE event_id IN ("+placeholders+")", args...)
		if err != nil {
			return fmt.Errorf("update event log success: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read event log success count: %w", err)
		}
		if changed != int64(len(batch)) {
			// Already-completed retries may change zero rows; missing logs must still fail.
			check := []any{StatusSuccess}
			check = append(check, args[2:]...)
			var present int
			if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_event_log WHERE status=? AND event_id IN ("+placeholders+")", check...).Scan(&present); err != nil {
				return fmt.Errorf("verify event log success: %w", err)
			}
			if present != len(batch) {
				return fmt.Errorf("event log success missing records: got %d, want %d", present, len(batch))
			}
		}
	}
	return nil
}
