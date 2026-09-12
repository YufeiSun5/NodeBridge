package syncstore

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) IsRepairReplay(ctx context.Context, eventID, node, database, table, operation string) (bool, error) {
	if s.DB == nil {
		return false, errors.New("repair replay database is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var found int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM sync_repair_replay WHERE event_id=? AND node_id=? AND database_name=? AND table_name=? AND (operation=? OR (?='UPDATE' AND operation='DELETE')) FOR SHARE", eventID, node, database, table, operation, operation).Scan(&found)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return found == 1, nil
}
