package conflict

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// RowKey must be canonicalized using the actual target primary key and collation.
// Raw source JSON is not a canonical SQL row identity.
type RowKey struct {
	Database     string `json:"database"`
	Table        string `json:"table"`
	CanonicalKey []byte `json:"canonical_key"`
}

type Store struct{ DB *sql.DB }

type Write func(context.Context, *sql.Tx) error
type Receipt func(context.Context, *sql.Tx, string) error

// Apply commits the winning business write, version/tombstone and event receipt together.
// Callers may ACK only after success, and must register uncaptured local writes first.
func (s Store) Apply(ctx context.Context, key RowKey, incoming Version, write Write, receipt Receipt) (string, error) {
	if s.DB == nil || write == nil || receipt == nil {
		return "", errors.New("conflict_transaction_dependencies_required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	decision, err := ApplyInTx(ctx, tx, key, incoming, write, receipt)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit conflict transaction (outcome may be unknown): %w", err)
	}
	return decision, nil
}

// ApplyInTx never commits or rolls back the caller's transaction. On error the
// caller must roll back the entire transaction; success is not permission to ACK.
// Register uncaptured local writes before calling and use a canonical SQL key.
func ApplyInTx(ctx context.Context, tx *sql.Tx, key RowKey, incoming Version, write Write, receipt Receipt) (string, error) {
	if tx == nil || write == nil || receipt == nil {
		return "", errors.New("conflict_transaction_dependencies_required")
	}
	if key.Database == "" || key.Table == "" || len(key.CanonicalKey) == 0 || len(key.CanonicalKey) > 4096 {
		return "", errors.New("conflict_canonical_row_key_required")
	}
	if err := incoming.Validate(); err != nil {
		return "", err
	}
	identity, err := json.Marshal(key)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(identity)
	// The unique key serializes first-writer races as well as updates to existing rows.
	if _, err := tx.ExecContext(ctx, "INSERT INTO sync_row_version (row_hash,row_identity,version_json) VALUES (?,?,NULL) ON DUPLICATE KEY UPDATE row_hash=row_hash", digest[:], identity); err != nil {
		return "", fmt.Errorf("lock conflict row: %w", err)
	}
	var storedIdentity, storedVersion []byte
	if err := tx.QueryRowContext(ctx, "SELECT row_identity,version_json FROM sync_row_version WHERE row_hash=? FOR UPDATE", digest[:]).Scan(&storedIdentity, &storedVersion); err != nil {
		return "", err
	}
	if !bytes.Equal(storedIdentity, identity) {
		return "", errors.New("conflict_row_hash_collision")
	}
	seen, err := checkSeen(ctx, tx, digest[:], incoming)
	if err != nil {
		return "", err
	}
	if seen {
		if err := receipt(ctx, tx, Duplicate); err != nil {
			return "", err
		}
		return Duplicate, nil
	}
	var current *Version
	if len(storedVersion) > 0 {
		current = &Version{}
		if err := json.Unmarshal(storedVersion, current); err != nil {
			return "", fmt.Errorf("decode conflict version: %w", err)
		}
	}
	decision, err := Resolve(current, incoming)
	if err != nil {
		return "", err
	}
	if decision == Apply {
		if err := write(ctx, tx); err != nil {
			return "", err
		}
		encoded, err := json.Marshal(incoming)
		if err != nil {
			return "", err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE sync_row_version SET version_json=? WHERE row_hash=?", string(encoded), digest[:]); err != nil {
			return "", err
		}
	}
	if err := saveSeen(ctx, tx, digest[:], incoming, decision); err != nil {
		return "", err
	}
	if err := receipt(ctx, tx, decision); err != nil {
		return "", err
	}
	return decision, nil
}
