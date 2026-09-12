package conflict_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/conflict"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

// The SQL-only fixture has no concurrent source writer. Real fencing is tested separately.
type quietSQLFixtureFence struct{}

func (quietSQLFixtureFence) Wait(ctx context.Context) error { return ctx.Err() }

func verifySoftConflictRepair(t *testing.T, ctx context.Context, db *sql.DB, database string) {
	t.Helper()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE TABLE soft_target (id BIGINT PRIMARY KEY,value VARCHAR(30),raw_bytes VARBINARY(8),is_deleted TINYINT NOT NULL,deleted_at DATETIME(6),deleted_by_node VARCHAR(64),last_event_id VARCHAR(128),updated_by_node VARCHAR(64)) ENGINE=InnoDB")
	row := map[string]any{"id": "1", "value": "removed", "raw_bytes": []byte{0, 128, 255}, "is_deleted": 0, "deleted_at": nil, "deleted_by_node": "", "last_event_id": "", "updated_by_node": ""}
	evt := event.SyncEvent{EventID: "soft-delete", OriginNodeID: "soft-remote", SourceNodeID: "soft-remote", DatabaseName: "remote", TableName: "source", EventType: event.TypeDelete, EventTime: time.Unix(3000, 123456000).UTC(), BinlogFile: "mysql-bin.000001", BinlogPos: 4, Headers: map[string]string{"event_time_source": conflict.SourceBinlog}, PrimaryKey: map[string]any{"id": "1"}, Before: row}
	rule := rules.SyncRule{PrimaryKeys: []string{"id"}, TargetDatabaseName: database, TargetTableName: "soft_target", Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin, DeleteMode: rules.DeleteSoft}
	worker := apply.NewCheckedSQLWorker(db)
	worker.CaptureFence = quietSQLFixtureFence{}
	mapped, err := mapper.MapEvent(evt, rule)
	if err != nil {
		t.Fatal(err)
	}
	exec("CREATE TRIGGER reject_winner_state BEFORE INSERT ON sync_conflict_state FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned winner image failure'")
	if _, err := worker.Apply(ctx, mapped); err == nil {
		t.Fatal("winner image failure accepted")
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM soft_target").Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed image save did not roll back business row: %d %v", count, err)
	}
	exec("DROP TRIGGER reject_winner_state")
	if result, err := worker.Apply(ctx, mapped); err != nil || result.ConflictDecision != conflict.Apply {
		t.Fatalf("soft delete missing row: %+v %v", result, err)
	}
	check := func() {
		t.Helper()
		var value, raw, at string
		var deleted int
		if err := db.QueryRowContext(ctx, "SELECT value,HEX(raw_bytes),is_deleted,DATE_FORMAT(deleted_at,'%Y-%m-%d %H:%i:%s.%f') FROM soft_target WHERE id=1").Scan(&value, &raw, &deleted, &at); err != nil || value != "removed" || raw != "0080FF" || deleted != 1 || at != "1970-01-01 00:50:00.123456" {
			t.Fatalf("soft winner: %s %s %d %s %v", value, raw, deleted, at, err)
		}
	}
	check()
	set := &rules.RuleSet{Rules: []rules.SyncRule{{ID: "soft-local", Enable: true, DatabaseName: database, TableName: "soft_target", PrimaryKeys: []string{"id"}, Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin, DeleteMode: rules.DeleteSoft}}}
	recorder := conflict.LocalRecorder{DB: db, NodeID: "soft-local", Rules: set}
	repair := apply.RepairWorker{DB: db, NodeID: "soft-local", Rules: set, CaptureFence: quietSQLFixtureFence{}}
	local := evt
	local.EventID, local.OriginNodeID, local.SourceNodeID, local.DatabaseName, local.TableName = "soft-local-update", "soft-local", "soft-local", database, "soft_target"
	local.EventType, local.EventTime, local.After = event.TypeUpdate, evt.EventTime.Add(-time.Second), row
	exec("UPDATE soft_target SET value='local-loser',raw_bytes=X'FF',is_deleted=0 WHERE id=1")
	if err := recorder.RecordLocal(ctx, local); err != nil {
		t.Fatal(err)
	}
	if repaired, err := repair.RunOnce(ctx); err != nil || !repaired {
		t.Fatalf("soft repair %t %v", repaired, err)
	}
	check()
	exec("DELETE FROM soft_target WHERE id=1")
	local.EventID, local.EventType, local.After = "soft-local-delete", event.TypeDelete, nil
	if err := recorder.RecordLocal(ctx, local); err != nil {
		t.Fatal(err)
	}
	if repaired, err := repair.RunOnce(ctx); err != nil || !repaired {
		t.Fatalf("missing soft repair %t %v", repaired, err)
	}
	check()
	t.Log("PASS: SQL soft tombstone creation for absent row, atomic winner-image failure rollback, source deletion timestamp, exact soft-row repair and restoration after local physical deletion; no CDC claimed for this SQL-only case")
}
