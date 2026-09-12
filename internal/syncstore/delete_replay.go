package syncstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *Store) IsDeleteReplay(ctx context.Context, eventID, originNodeID, database, table string) (bool, error) {
	if s.DB == nil {
		return false, errors.New("sync store db is required")
	}
	if eventID == "" || originNodeID == "" || database == "" || table == "" {
		return false, errors.New("delete replay identity is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var found int
	// Locking read waits if Canal sees binlog before InnoDB completes commit.
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM sync_delete_replay WHERE event_id=? AND origin_node_id=? AND database_name=? AND table_name=? FOR SHARE", eventID, originNodeID, database, table).Scan(&found)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("read delete replay: %w", err)
	}
	exists := err == nil
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return exists, nil
}
