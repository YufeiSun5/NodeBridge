package conflict

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
)

func eventIdentity(v Version) []byte {
	value, _ := json.Marshal([]string{v.OriginNodeID, v.EventID})
	return value
}

// LookupDecision uses the indexed immutable receipt, not the current row winner.
func LookupDecision(ctx context.Context, db *sql.DB, origin, eventID string) (string, bool, error) {
	if db == nil || origin == "" || eventID == "" {
		return "", false, errors.New("conflict_receipt_identity_required")
	}
	identity := eventIdentity(Version{OriginNodeID: origin, EventID: eventID})
	hash := sha256.Sum256(identity)
	var stored []byte
	var decision string
	err := db.QueryRowContext(ctx, "SELECT event_identity,decision FROM sync_conflict_event WHERE event_hash=?", hash[:]).Scan(&stored, &decision)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !bytes.Equal(stored, identity) || (decision != Apply && decision != Superseded) {
		return "", false, errors.New("conflict_receipt_invalid")
	}
	return decision, true, nil
}

func checkSeen(ctx context.Context, tx *sql.Tx, rowHash []byte, incoming Version) (bool, error) {
	identity := eventIdentity(incoming)
	hash := sha256.Sum256(identity)
	var storedRow, storedIdentity, encoded []byte
	err := tx.QueryRowContext(ctx, "SELECT row_hash,event_identity,version_json FROM sync_conflict_event WHERE event_hash=? FOR UPDATE", hash[:]).Scan(&storedRow, &storedIdentity, &encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !bytes.Equal(storedIdentity, identity) {
		return false, errors.New("conflict_event_hash_collision")
	}
	if !bytes.Equal(storedRow, rowHash) {
		return false, errors.New("conflict_event_identity_reused: canonical row changed")
	}
	var stored Version
	if err := json.Unmarshal(encoded, &stored); err != nil {
		return false, err
	}
	decision, err := Resolve(&stored, incoming)
	if err != nil {
		return false, err
	}
	if decision != Duplicate {
		return false, errors.New("conflict_event_identity_reused")
	}
	return true, nil
}

func saveSeen(ctx context.Context, tx *sql.Tx, rowHash []byte, incoming Version, decision string) error {
	identity := eventIdentity(incoming)
	hash := sha256.Sum256(identity)
	encoded, err := json.Marshal(incoming)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO sync_conflict_event (row_hash,event_hash,event_identity,version_json,decision) VALUES (?,?,?,?,?)", rowHash, hash[:], identity, string(encoded), decision)
	return err
}
