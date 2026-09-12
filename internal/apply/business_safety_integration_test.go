package apply_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
)

// This test never accepts an existing database or a remote endpoint.
func TestBusinessSafetyRealMySQL(t *testing.T) {
	raw := os.Getenv("NODEBRIDGE_BUSINESS_SAFETY_TEST_DSN")
	if raw == "" {
		t.Skip("NODEBRIDGE_BUSINESS_SAFETY_TEST_DSN required")
	}
	cfg, err := mysql.ParseDSN(raw)
	if err != nil {
		t.Fatal("invalid test DSN")
	}
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil || cfg.Net != "tcp" || cfg.DBName != "" || (host != "127.0.0.1" && host != "::1") {
		t.Fatal("test requires loopback TCP and an empty database name")
	}
	cfg.ParseTime = true
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 5*time.Second, 10*time.Second, 10*time.Second
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	name := fmt.Sprintf("nb_business_fix_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"`"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if _, err := admin.ExecContext(cleanup, "DROP DATABASE `"+name+"`"); err != nil {
			t.Errorf("cleanup owned database %s: %v", name, err)
		}
	}()
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := mysqlconn.RunMigrations(ctx, db, "../../migrations/server"); err != nil {
		t.Fatal(err)
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE TABLE safety_rows (id BIGINT PRIMARY KEY, code VARCHAR(40) UNIQUE, value INT NOT NULL) ENGINE=InnoDB")
	exec("CREATE TABLE safety_child (id BIGINT PRIMARY KEY, parent_id BIGINT, FOREIGN KEY (parent_id) REFERENCES safety_rows(id)) ENGINE=InnoDB")
	worker := apply.NewCheckedSQLWorker(db)
	makeEvent := func(id int64, eid, op, code string, value int) mapper.MappedEvent {
		pk := map[string]any{"id": id}
		after := map[string]any{"id": id, "code": code, "value": value}
		return mapper.MappedEvent{
			Event:          event.SyncEvent{EventID: eid, OriginNodeID: "edge-test", SourceNodeID: "edge-test", EventType: op, PrimaryKey: pk, After: after},
			SourceDatabase: name, SourceTable: "source_rows", TargetDatabase: name, TargetTable: "safety_rows",
			TargetPrimaryKey: pk, TargetKeyColumns: []string{"id"}, TargetAfter: after, DeleteMode: rules.DeleteHard,
		}
	}
	check := func(evt mapper.MappedEvent, code string) apply.Result {
		t.Helper()
		result, err := worker.Apply(ctx, evt)
		if code == "" && err != nil || code != "" && (err == nil || !strings.Contains(err.Error(), code)) {
			t.Fatalf("event %s expected %q: %v", evt.Event.EventID, code, err)
		}
		var receipts int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_apply_log WHERE event_id=?", evt.Event.EventID).Scan(&receipts); err != nil {
			t.Fatal(err)
		}
		if (code == "" && receipts != 1) || (code != "" && receipts != 0) {
			t.Fatalf("event %s receipts=%d", evt.Event.EventID, receipts)
		}
		return result
	}
	const largeID int64 = 9007199254740993
	seed := makeEvent(largeID, "seed", event.TypeInsert, "unique-a", 1)
	check(seed, "")
	if !check(seed, "").AlreadyApplied {
		t.Fatal("duplicate event was not idempotent")
	}
	exec("INSERT INTO safety_child VALUES (1, ?)", largeID)
	check(makeEvent(largeID+1, "collision", event.TypeInsert, "unique-a", 9), "unique_key_conflict")
	var actualID int64
	var value int
	if err := db.QueryRowContext(ctx, "SELECT id,value FROM safety_rows WHERE code='unique-a'").Scan(&actualID, &value); err != nil || actualID != largeID || value != 1 {
		t.Fatalf("conflict changed existing row: id=%d value=%d err=%v", actualID, value, err)
	}
	check(makeEvent(largeID, "noop", event.TypeUpdate, "unique-a", 1), "")
	missing := makeEvent(20, "missing", event.TypeUpdate, "repair", 2)
	check(missing, "target_row_missing")
	exec("INSERT INTO safety_rows VALUES (20,'repair',1)")
	check(missing, "")
	check(makeEvent(largeID, "fk-delete", event.TypeDelete, "", 0), "apply hard delete")
	exec("DELETE FROM safety_child WHERE id=1")
	check(makeEvent(largeID, "hard", event.TypeDelete, "", 0), "")
	check(makeEvent(largeID, "hard-absent", event.TypeDelete, "", 0), "")
	batch := []mapper.MappedEvent{
		makeEvent(30, "prefix", event.TypeInsert, "prefix", 1),
		makeEvent(31, "blocked", event.TypeUpdate, "blocked", 2),
		makeEvent(32, "suffix", event.TypeInsert, "suffix", 1),
	}
	result, err := worker.ApplyBatch(ctx, batch)
	if err == nil || !strings.Contains(err.Error(), "target_row_missing") || len(result.Results) != 1 {
		t.Fatalf("prefix result=%+v err=%v", result, err)
	}
	var suffix int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM safety_rows WHERE id=32").Scan(&suffix); err != nil || suffix != 0 {
		t.Fatalf("suffix committed: count=%d err=%v", suffix, err)
	}
	exec("INSERT INTO safety_rows VALUES (31,'blocked',1)")
	result, err = worker.ApplyBatch(ctx, batch)
	if err != nil || len(result.Results) != 3 || !result.Results[0].AlreadyApplied {
		t.Fatalf("recovery result=%+v err=%v", result, err)
	}
	// A receipt failure must roll back the preceding business DELETE.
	exec("CREATE TRIGGER reject_receipt BEFORE INSERT ON sync_apply_log FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned receipt failure'")
	check(makeEvent(20, "receipt-failure", event.TypeDelete, "", 0), "insert apply log")
	if err := db.QueryRowContext(ctx, "SELECT value FROM safety_rows WHERE id=20").Scan(&value); err != nil || value != 2 {
		t.Fatalf("delete not rolled back: value=%d err=%v", value, err)
	}
	exec("DROP TRIGGER reject_receipt")
	exec("CREATE TABLE composite_rows (id BIGINT, tenant_id BIGINT, value INT, PRIMARY KEY(id,tenant_id)) ENGINE=InnoDB")
	exec("INSERT INTO composite_rows VALUES (1,10,1),(1,20,1)")
	partial := makeEvent(1, "partial-real-pk", event.TypeDelete, "", 0)
	partial.TargetTable = "composite_rows"
	check(partial, "primary_key_mismatch")
	var remaining int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM composite_rows").Scan(&remaining); err != nil || remaining != 2 {
		t.Fatalf("partial key changed rows: %d %v", remaining, err)
	}
	exec("CREATE TABLE soft_rows (id BIGINT PRIMARY KEY, is_deleted TINYINT NOT NULL DEFAULT 0, deleted_at DATETIME(6), deleted_by_node VARCHAR(100), updated_by_node VARCHAR(100), last_event_id VARCHAR(100)) ENGINE=InnoDB")
	exec("INSERT INTO soft_rows (id) VALUES (1)")
	soft := makeEvent(1, "soft-excluded-mapping", event.TypeDelete, "", 0)
	soft.TargetTable, soft.DeleteMode = "soft_rows", rules.DeleteSoft
	soft.TargetColumns = map[string]string{"excluded_source": "absent_target"}
	check(soft, "")
	var deleted int
	if err := db.QueryRowContext(ctx, "SELECT is_deleted FROM soft_rows WHERE id=1").Scan(&deleted); err != nil || deleted != 1 {
		t.Fatalf("soft delete failed: %d %v", deleted, err)
	}
	t.Logf("owned database=%s; strict INSERT/FK/64-bit ID/noop/missing UPDATE/retry/HARD/SOFT/receipt rollback/batch prefix verified", name)
}
