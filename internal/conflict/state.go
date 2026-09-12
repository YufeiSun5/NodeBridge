package conflict

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type RowState struct {
	Version    Version           `json:"version"`
	PrimaryKey map[string]any    `json:"primary_key"`
	Row        map[string]any    `json:"row"`
	Columns    map[string]string `json:"columns"`
	DeleteMode string            `json:"delete_mode"`
}

func rowIdentity(key RowKey) ([]byte, [32]byte) {
	identity, _ := json.Marshal(key)
	return identity, sha256.Sum256(identity)
}

func stateTableHash(database, table string) [32]byte {
	identity, _ := json.Marshal([]string{database, table})
	return sha256.Sum256(identity)
}

// SaveWinnerState is part of the version transaction, never a later best-effort write.
func SaveWinnerState(ctx context.Context, tx *sql.Tx, key RowKey, state RowState) error {
	if tx == nil || key.Database == "" || key.Table == "" || len(key.CanonicalKey) == 0 {
		return errors.New("conflict_winner_state_dependencies_required")
	}
	if err := state.Version.Validate(); err != nil {
		return err
	}
	if len(state.PrimaryKey) == 0 || (!state.Version.Deleted && len(state.Row) == 0) {
		return errors.New("conflict_winner_image_required")
	}
	if state.DeleteMode != rules.DeleteHard && state.DeleteMode != rules.DeleteSoft {
		return errors.New("conflict_winner_delete_mode_required")
	}
	if state.DeleteMode == rules.DeleteSoft && len(state.Row) == 0 {
		return errors.New("conflict_winner_image_required")
	}
	identity, hash := rowIdentity(key)
	tableHash := stateTableHash(key.Database, key.Table)
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO sync_conflict_state (row_hash,table_hash,row_identity,state_json,repair_required) VALUES (?,?,?,?,0) ON DUPLICATE KEY UPDATE state_json=?,repair_required=0", hash[:], tableHash[:], identity, string(encoded), string(encoded))
	return err
}

// ReadWinnerState locks the ledger before its image and verifies they agree.
func ReadWinnerState(ctx context.Context, tx *sql.Tx, key RowKey) (RowState, bool, error) {
	identity, hash := rowIdentity(key)
	var actualIdentity, encodedVersion []byte
	if err := tx.QueryRowContext(ctx, "SELECT row_identity,version_json FROM sync_row_version WHERE row_hash=? FOR UPDATE", hash[:]).Scan(&actualIdentity, &encodedVersion); err != nil {
		return RowState{}, false, err
	}
	if !bytes.Equal(identity, actualIdentity) {
		return RowState{}, false, errors.New("conflict_row_hash_collision")
	}
	var encoded []byte
	var repair bool
	if err := tx.QueryRowContext(ctx, "SELECT row_identity,state_json,repair_required FROM sync_conflict_state WHERE row_hash=? FOR UPDATE", hash[:]).Scan(&actualIdentity, &encoded, &repair); err != nil {
		return RowState{}, false, err
	}
	if !bytes.Equal(identity, actualIdentity) {
		return RowState{}, false, errors.New("conflict_row_hash_collision")
	}
	state, err := decodeState(encoded)
	if err != nil {
		return RowState{}, false, err
	}
	var version Version
	if err := json.Unmarshal(encodedVersion, &version); err != nil {
		return RowState{}, false, err
	}
	decision, err := Resolve(&version, state.Version)
	if err != nil {
		return RowState{}, false, err
	}
	if decision != Duplicate {
		return RowState{}, false, errors.New("conflict_winner_state_mismatch")
	}
	return state, repair, nil
}

func SetRepairRequired(ctx context.Context, tx *sql.Tx, key RowKey, required bool) error {
	if _, _, err := ReadWinnerState(ctx, tx, key); err != nil {
		return err
	}
	_, hash := rowIdentity(key)
	_, err := tx.ExecContext(ctx, "UPDATE sync_conflict_state SET repair_required=? WHERE row_hash=?", required, hash[:])
	return err
}

// NextRepair returns only a candidate. A worker must fence and reload it under locks.
func NextRepair(ctx context.Context, db *sql.DB) (RowKey, RowState, bool, error) {
	var identity, encoded []byte
	err := db.QueryRowContext(ctx, "SELECT row_identity,state_json FROM sync_conflict_state WHERE repair_required=1 ORDER BY row_hash LIMIT 1").Scan(&identity, &encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return RowKey{}, RowState{}, false, nil
	}
	if err != nil {
		return RowKey{}, RowState{}, false, err
	}
	var key RowKey
	if err := json.Unmarshal(identity, &key); err != nil {
		return key, RowState{}, false, err
	}
	state, err := decodeState(encoded)
	return key, state, err == nil, err
}

func NextTableRepair(ctx context.Context, db *sql.DB, database, table string) (RowKey, RowState, bool, error) {
	hash := stateTableHash(database, table)
	var identity, encoded []byte
	err := db.QueryRowContext(ctx, "SELECT row_identity,state_json FROM sync_conflict_state WHERE repair_required=1 AND table_hash=? ORDER BY row_hash LIMIT 1", hash[:]).Scan(&identity, &encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return RowKey{}, RowState{}, false, nil
	}
	if err != nil {
		return RowKey{}, RowState{}, false, err
	}
	var key RowKey
	if err := json.Unmarshal(identity, &key); err != nil {
		return key, RowState{}, false, err
	}
	if key.Database != database || key.Table != table {
		return key, RowState{}, false, errors.New("conflict_table_hash_collision")
	}
	state, err := decodeState(encoded)
	return key, state, err == nil, err
}

func decodeState(encoded []byte) (RowState, error) {
	var state RowState
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&state); err != nil {
		return state, err
	}
	if err := rowvalue.Restore(state.PrimaryKey, state.Row); err != nil {
		return state, err
	}
	return state, state.Version.Validate()
}
