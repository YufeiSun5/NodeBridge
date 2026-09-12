package conflict_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/conflict"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func verifySQLCanonicalKeysAndLocalRecorder(t *testing.T, ctx context.Context, db *sql.DB, database string) {
	t.Helper()
	for _, table := range []struct {
		name, definition string
		equal, different any
	}{
		{"key_int", "BIGINT UNSIGNED", "001", "2"},
		{"key_pad", "VARCHAR(30) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", "A ", "b"},
		{"key_accent", "VARCHAR(30) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", "\u00e1", "b"},
		{"key_combining", "VARCHAR(30) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci", "a\u0301", "b"},
		{"key_nopad", "VARCHAR(30) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci", "A", "a "},
		{"key_binary", "VARCHAR(30) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin", "a", "A"},
	} {
		if _, err := db.ExecContext(ctx, "CREATE TABLE "+table.name+" (id "+table.definition+" PRIMARY KEY, value INT NOT NULL DEFAULT 0) ENGINE=InnoDB"); err != nil {
			t.Fatal(err)
		}
		schema, err := rulecheck.ReadSchema(ctx, db, database, table.name)
		if err != nil {
			t.Fatal(err)
		}
		base := "a"
		if table.name == "key_int" {
			base = "1"
		}
		key := func(value any) conflict.RowKey {
			t.Helper()
			got, err := conflict.CanonicalKey(ctx, db, schema, map[string]any{"id": value})
			if err != nil {
				t.Fatal(err)
			}
			return got
		}
		original := key(base)
		if !bytes.Equal(original.CanonicalKey, key(table.equal).CanonicalKey) || bytes.Equal(original.CanonicalKey, key(table.different).CanonicalKey) {
			t.Fatalf("SQL key equivalence mismatch: %s", table.name)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO "+table.name+" (id) VALUES (?)", base); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table.name+" WHERE id=?", table.equal).Scan(&count); err != nil || count != 1 {
			t.Fatalf("actual SQL equality disagrees: %s %d %v", table.name, count, err)
		}
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table.name+" WHERE id=?", table.different).Scan(&count); err != nil || count != 0 {
			t.Fatalf("actual SQL inequality disagrees: %s %d %v", table.name, count, err)
		}
	}
	set := &rules.RuleSet{Rules: []rules.SyncRule{{ID: "local-test", Enable: true, DatabaseName: database, TableName: "key_int", Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin, PrimaryKeys: []string{"id"}}}}
	recorder := conflict.LocalRecorder{DB: db, NodeID: "local", Rules: set}
	evt := event.SyncEvent{EventID: "local-captured-1", OriginNodeID: "local", SourceNodeID: "local", DatabaseName: database, TableName: "key_int", EventType: event.TypeUpdate, EventTime: time.Unix(2000, 0), BinlogFile: "mysql-bin.000001", BinlogPos: 4, Headers: map[string]string{"event_time_source": conflict.SourceBinlog}, PrimaryKey: map[string]any{"id": "001"}, Before: map[string]any{"id": "1", "value": "0"}, After: map[string]any{"id": "1", "value": "5"}}
	if _, err := db.ExecContext(ctx, "UPDATE key_int SET value=5 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	locked, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer locked.Rollback()
	var value int
	if err := locked.QueryRowContext(ctx, "SELECT value FROM key_int WHERE id=1 FOR UPDATE").Scan(&value); err != nil {
		t.Fatal(err)
	}
	deadline, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := recorder.RecordLocal(deadline, evt); err != nil {
		t.Fatalf("local registration waited on business lock: %v", err)
	}
	if err := recorder.RecordLocal(deadline, evt); err != nil {
		t.Fatal(err)
	}
	if err := locked.Rollback(); err != nil {
		t.Fatal(err)
	}
	var encoded []byte
	if err := db.QueryRowContext(ctx, "SELECT version_json FROM sync_row_version WHERE JSON_UNQUOTE(JSON_EXTRACT(version_json,'$.event_id'))=?", evt.EventID).Scan(&encoded); err != nil {
		t.Fatal(err)
	}
	var stored conflict.Version
	if err := json.Unmarshal(encoded, &stored); err != nil || stored.EventID != evt.EventID {
		t.Fatalf("local version not durable: %s %v", encoded, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT value FROM key_int WHERE id=1").Scan(&value); err != nil || value != 5 {
		t.Fatalf("recorder rewrote business: %d %v", value, err)
	}
	older := evt
	older.EventID, older.EventTime = "uncaptured-older-local", evt.EventTime.Add(-time.Second)
	if err := recorder.RecordLocal(ctx, older); err != nil {
		t.Fatal(err)
	}
	if _, _, pending, err := conflict.NextTableRepair(ctx, db, database, "key_int"); err != nil || !pending {
		t.Fatalf("local loser did not create durable repair: %t %v", pending, err)
	}
	later := evt
	later.EventID, later.EventTime, later.BinlogPos = "local-captured-newer", evt.EventTime.Add(time.Second), 10
	later.After = map[string]any{"id": "1", "value": "6"}
	if _, err := db.ExecContext(ctx, "UPDATE key_int SET value=6 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if err := recorder.RecordLocal(ctx, later); err != nil {
		t.Fatal(err)
	}
	if _, _, pending, err := conflict.NextTableRepair(ctx, db, database, "key_int"); err != nil || pending {
		t.Fatalf("new local winner did not cancel obsolete repair: %t %v", pending, err)
	}
	evt = later
	evt.EventID, evt.EventType, evt.BinlogPos = "local-delete", event.TypeDelete, 5
	evt.EventTime = evt.EventTime.Add(time.Second)
	evt.Before, evt.After = evt.After, nil
	if _, err := db.ExecContext(ctx, "DELETE FROM key_int WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if err := recorder.RecordLocal(ctx, evt); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE key_int MODIFY id DECIMAL(20,0) NOT NULL"); err != nil {
		t.Fatal(err)
	}
	evt.EventID = "after-key-ddl"
	if err := recorder.RecordLocal(ctx, evt); err == nil || !strings.Contains(err.Error(), "key_schema_changed") {
		t.Fatalf("key schema drift accepted: %v", err)
	}
	t.Log("PASS: actual MySQL numeric/collation/PAD primary-key equivalence; durable idempotent local version registration without business row locks or rewrites; delete capture and key-schema drift refusal")
}
