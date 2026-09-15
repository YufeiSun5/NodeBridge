// Package replay identifies sync writes using transactional system-table markers.
package replay

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
)

const Table = "sync_replay_marker"

// Begin and End must bracket writes in the same transaction. MySQL serializes
// committed transaction contents in the binlog, even with concurrent writers.
func Begin(ctx context.Context, tx *sql.Tx, database, table string) (string, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(random[:])
	return token, Mark(ctx, tx, token, "BEGIN", database, table)
}

func End(ctx context.Context, tx *sql.Tx, token, database, table string) error {
	return Mark(ctx, tx, token, "END", database, table)
}

func Mark(ctx context.Context, tx *sql.Tx, token, phase, database, table string) error {
	if tx == nil || len(token) != 64 || database == "" || table == "" {
		return errors.New("replay_marker_invalid")
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO sync_replay_marker (token,phase,database_name,table_name) VALUES (?,?,?,?)", token, phase, database, table)
	return err
}
