package conflict

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
)

type keyColumn struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Collation string `json:"collation"`
}

type pinnedKeySchema struct {
	Codec   int         `json:"codec"`
	Columns []keyColumn `json:"columns"`
}

// PinKeySchema rejects changes that would give existing rows new ledger identities.
// The caller must roll back on error and must not hold this lock across a CDC wait.
func PinKeySchema(ctx context.Context, tx *sql.Tx, schema rulecheck.Schema) error {
	if tx == nil || len(schema.PrimaryKeys) == 0 {
		return errors.New("conflict_actual_primary_key_required")
	}
	pinned := pinnedKeySchema{Codec: 1}
	for _, name := range schema.PrimaryKeys {
		found := false
		for _, c := range schema.Columns {
			if c.Name == name {
				pinned.Columns = append(pinned.Columns, keyColumn{Name: c.Name, Type: c.Type, Collation: c.Collation})
				found = true
				break
			}
		}
		if !found {
			return errors.New("conflict_actual_primary_key_required")
		}
	}
	identity, _ := json.Marshal([]string{schema.Database, schema.Table})
	hash := sha256.Sum256(identity)
	encoded, _ := json.Marshal(pinned)
	if _, err := tx.ExecContext(ctx, "INSERT INTO sync_conflict_schema (table_hash,table_identity,key_schema) VALUES (?,?,?) ON DUPLICATE KEY UPDATE table_hash=table_hash", hash[:], identity, string(encoded)); err != nil {
		return err
	}
	var storedIdentity, stored []byte
	if err := tx.QueryRowContext(ctx, "SELECT table_identity,key_schema FROM sync_conflict_schema WHERE table_hash=? FOR UPDATE", hash[:]).Scan(&storedIdentity, &stored); err != nil {
		return err
	}
	if !bytes.Equal(identity, storedIdentity) {
		return errors.New("conflict_table_hash_collision")
	}
	var actual pinnedKeySchema
	if err := json.Unmarshal(stored, &actual); err != nil {
		return err
	}
	if !reflect.DeepEqual(pinned, actual) {
		return errors.New("conflict_key_schema_changed: explicit ledger migration is required")
	}
	return nil
}
