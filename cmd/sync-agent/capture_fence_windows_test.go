//go:build windows

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/capture"
	"github.com/YufeiSun5/NodeBridge/internal/cdc/canal"
	"github.com/YufeiSun5/NodeBridge/internal/conflict"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/normalizer"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncruntime"
	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
)

type captureRecorder struct {
	mu     sync.Mutex
	events []event.SyncEvent
}

func (r *captureRecorder) Publish(_ context.Context, request rabbitmq.PublishRequest) error {
	var evt event.SyncEvent
	if err := json.Unmarshal(request.Body, &evt); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, evt)
	return nil
}

// Called only inside the owned fixture, after both candidate processes stop.
func verifyOwnedCaptureFence(t *testing.T, parent context.Context, db *sql.DB, address string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	cfg := canal.Config{ReaderName: "owned-fence-reader", Address: address, Destination: "example", Filter: `nb_cdc_source\.(source_rows|sync_capture_fence)`, BatchSize: 16}
	client, err := canal.NewWithlinClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := canal.NewAdapter(cfg, client, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Internal fixture only: production still rejects enabled BIDIRECTIONAL rules.
	set := &rules.RuleSet{Rules: []rules.SyncRule{{ID: "owned-fence", Enable: true, DatabaseName: "nb_cdc_source", TableName: "source_rows", PrimaryKeys: []string{"source_id"}, Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin}}}
	appCfg := &appconfig.Config{}
	appCfg.Node.ID, appCfg.MySQL.Database, appCfg.CDC.Type = "owned-fence", "nb_cdc_source", "canal"
	worker := apply.NewCheckedSQLWorker(db)
	source, localVersions, repairRuntime, err := attachConflictRuntime(appCfg, set, db, worker, adapter)
	if err != nil || repairRuntime == nil {
		t.Fatalf("attach production conflict components: %v", err)
	}
	fence := worker.CaptureFence.(*capture.Fence)
	recorder := &captureRecorder{}
	runtime := &syncruntime.CanalUploadRuntime{Source: source, LocalVersions: localVersions, Normalizer: normalizer.New(normalizer.Options{NodeID: "owned-fence"}), Publisher: recorder}
	runtime.Decider = loop.NewSuppressor("owned-fence", *set, syncstore.New(db))
	done := make(chan error, 1)
	go func() {
		defer runtime.Stop(context.Background())
		for ctx.Err() == nil {
			if _, err := runtime.RunOnce(ctx); err != nil {
				done <- err
				return
			}
		}
		done <- ctx.Err()
	}()
	defer func() {
		cancel()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("capture runtime: %v", err)
		}
	}()
	const key uint64 = 9007199254741999
	if _, err := db.ExecContext(ctx, "INSERT INTO source_rows (source_id,amount,optional_text,raw_bytes,changed_at) VALUES (?,1,'before-fence',X'00','2026-09-11 08:09:10.123456')", key); err != nil {
		t.Fatal(err)
	}
	lock, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	var found uint64
	if err := lock.QueryRowContext(ctx, "SELECT source_id FROM source_rows WHERE source_id=? FOR UPDATE", key).Scan(&found); err != nil {
		t.Fatal(err)
	}
	if err := fence.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	recorder.mu.Lock()
	seen := false
	var capturedID string
	for _, evt := range recorder.events {
		if evt.TableName == capture.Table {
			t.Error("internal pulse was published")
		}
		if evt.TableName == "source_rows" && evt.After["optional_text"] == "before-fence" {
			seen = true
			capturedID = evt.EventID
		}
	}
	recorder.mu.Unlock()
	if !seen {
		t.Fatal("fence released before earlier business change was processed")
	}
	var registered int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_row_version WHERE JSON_UNQUOTE(JSON_EXTRACT(version_json,'$.event_id'))=?", capturedID).Scan(&registered); err != nil || registered != 1 {
		t.Fatalf("barrier released without durable local version: %d %v", registered, err)
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	// A second pulse without business writes must not create SQL checkpoint feedback.
	before, _, err := adapter.Store.Load(ctx, cfg.ReaderName)
	if err != nil {
		t.Fatal(err)
	}
	if err := fence.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	after, _, err := adapter.Store.Load(ctx, cfg.ReaderName)
	if err != nil || before != after {
		t.Fatalf("pulse persisted a checkpoint: before=%+v after=%+v error=%v", before, after, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_capture_fence WHERE node_id='owned-fence'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("pulse storage unbounded: %d %v", count, err)
	}
	verifyOwnedIncomingConflict(t, ctx, db, fence, worker, key)
	t.Log("PASS: real Canal fence waits for durable local version while business row is locked; pulse-only batch releases without checkpoint feedback, internal pulse is not published; production constructors remain gated")
}

func verifyOwnedIncomingConflict(t *testing.T, ctx context.Context, db *sql.DB, fence *capture.Fence, worker *apply.SQLWorker, key uint64) {
	t.Helper()
	rule := rules.SyncRule{ID: "owned-incoming-conflict", DatabaseName: "owned_remote", TableName: "remote_rows", TargetDatabaseName: "nb_cdc_source", TargetTableName: "source_rows", PrimaryKeys: []string{"remote_id"}, Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin, DeleteMode: rules.DeleteHard, ColumnMappings: []rules.ColumnMapping{{SourceColumn: "remote_id", TargetColumn: "source_id"}, {SourceColumn: "remote_text", TargetColumn: "optional_text"}}}
	var encoded []byte
	if err := db.QueryRowContext(ctx, "SELECT version_json FROM sync_row_version ORDER BY JSON_UNQUOTE(JSON_EXTRACT(version_json,'$.time')) DESC LIMIT 1").Scan(&encoded); err != nil {
		t.Fatal(err)
	}
	var localVersion conflict.Version
	if err := json.Unmarshal(encoded, &localVersion); err != nil {
		t.Fatal(err)
	}
	base := localVersion.Time.Add(2 * time.Second)
	makeEvent := func(id, operation, text string, delta time.Duration) event.SyncEvent {
		row := map[string]any{"remote_id": key, "amount": "1.000000", "remote_text": text, "raw_bytes": []byte{0}, "changed_at": "2026-09-11 08:09:10.123456", "last_event_id": "", "updated_by_node": ""}
		evt := event.SyncEvent{EventID: id, OriginNodeID: "owned-remote-conflict", SourceNodeID: "owned-remote-conflict", DatabaseName: rule.DatabaseName, TableName: rule.TableName, EventType: operation, EventTime: base.Add(delta), BinlogFile: "mysql-bin.000020", BinlogPos: 100, Headers: map[string]string{"event_time_source": conflict.SourceBinlog}, PrimaryKey: map[string]any{"remote_id": key}}
		if operation == event.TypeDelete {
			evt.Before = row
		} else {
			evt.After = row
		}
		return evt
	}
	applyEvent := func(evt event.SyncEvent, want string) {
		t.Helper()
		mapped, err := mapper.MapEvent(evt, rule)
		if err != nil {
			t.Fatal(err)
		}
		result, err := worker.Apply(ctx, mapped)
		if err != nil || result.ConflictDecision != want {
			t.Fatalf("conflict apply %s: %+v %v", evt.EventID, result, err)
		}
	}
	newer := makeEvent("owned-conflict-winner", event.TypeInsert, "remote-winner", 0)
	applyEvent(newer, conflict.Apply)
	applyEvent(newer, conflict.Duplicate)
	applyEvent(makeEvent("owned-conflict-older", event.TypeUpdate, "must-not-win", -time.Second), conflict.Superseded)
	var got string
	if err := db.QueryRowContext(ctx, "SELECT optional_text FROM source_rows WHERE source_id=?", key).Scan(&got); err != nil || got != "remote-winner" {
		t.Fatalf("older write overwrote winner: %s %v", got, err)
	}
	if _, err := db.ExecContext(ctx, "CREATE TRIGGER reject_conflict_receipt BEFORE INSERT ON sync_apply_log FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned conflict rollback'"); err != nil {
		t.Fatal(err)
	}
	retry := makeEvent("owned-conflict-retry", event.TypeUpdate, "after-retry", time.Second)
	mapped, err := mapper.MapEvent(retry, rule)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.Apply(ctx, mapped); err == nil {
		t.Fatal("receipt failure accepted")
	}
	if err := db.QueryRowContext(ctx, "SELECT optional_text FROM source_rows WHERE source_id=?", key).Scan(&got); err != nil || got != "remote-winner" {
		t.Fatalf("failed apply not rolled back: %s %v", got, err)
	}
	if _, err := db.ExecContext(ctx, "DROP TRIGGER reject_conflict_receipt"); err != nil {
		t.Fatal(err)
	}
	applyEvent(retry, conflict.Apply)
	applyEvent(makeEvent("owned-conflict-delete", event.TypeDelete, "after-retry", 2*time.Second), conflict.Apply)
	applyEvent(makeEvent("owned-conflict-old-insert", event.TypeInsert, "must-not-resurrect", time.Second), conflict.Superseded)
	if err := fence.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM source_rows WHERE source_id=?", key).Scan(&count); err != nil || count != 0 {
		t.Fatalf("old insert resurrected tombstone: %d %v", count, err)
	}
	applyEvent(makeEvent("owned-conflict-revive", event.TypeUpdate, "revived", 3*time.Second), conflict.Apply)
	if err := db.QueryRowContext(ctx, "SELECT optional_text FROM source_rows WHERE source_id=?", key).Scan(&got); err != nil || got != "revived" {
		t.Fatalf("newer UPDATE did not restore deleted row: %s %v", got, err)
	}
	missing := makeEvent("owned-conflict-missing-delete", event.TypeDelete, "absent", 4*time.Second)
	missing.PrimaryKey["remote_id"], missing.Before["remote_id"] = key+1, key+1
	applyEvent(missing, conflict.Apply)
	missingInsert := makeEvent("owned-conflict-missing-old", event.TypeInsert, "must-not-appear", 3*time.Second)
	missingInsert.PrimaryKey["remote_id"], missingInsert.After["remote_id"] = key+1, key+1
	applyEvent(missingInsert, conflict.Superseded)
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM source_rows WHERE source_id=?", key+1).Scan(&count); err != nil || count != 0 {
		t.Fatalf("absent-row tombstone was lost: %d %v", count, err)
	}
	var batch []mapper.MappedEvent
	for i, text := range []string{"batch-first", "batch-second"} {
		evt := makeEvent("owned-conflict-"+text, event.TypeUpdate, text, time.Duration(5+i)*time.Second)
		mapped, err := mapper.MapEvent(evt, rule)
		if err != nil {
			t.Fatal(err)
		}
		batch = append(batch, mapped)
	}
	result, err := worker.ApplyBatch(ctx, batch)
	if err != nil || len(result.Results) != 2 {
		t.Fatalf("conflict batch: %+v %v", result, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT optional_text FROM source_rows WHERE source_id=?", key).Scan(&got); err != nil || got != "batch-second" {
		t.Fatalf("conflict batch order lost: %s %v", got, err)
	}
	newer.After["remote_text"] = "reused-old-id"
	reused, err := mapper.MapEvent(newer, rule)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.Apply(ctx, reused); err == nil {
		t.Fatal("historical event identity reuse accepted by Apply")
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_apply_log WHERE origin_node_id='owned-remote-conflict'").Scan(&count); err != nil || count != 10 {
		t.Fatalf("conflict receipts: %d %v", count, err)
	}
	set := &rules.RuleSet{Rules: []rules.SyncRule{{ID: "owned-repair", Enable: true, DatabaseName: "nb_cdc_source", TableName: "source_rows", PrimaryKeys: []string{"source_id"}, Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin}}}
	repair := apply.RepairWorker{DB: db, NodeID: "owned-fence", Rules: set, CaptureFence: fence}
	winner := makeEvent("owned-repair-winner", event.TypeUpdate, "repair-winner", time.Hour)
	winner.After["amount"], winner.After["raw_bytes"] = "123456789012345678901234.123456", []byte{0, 128, 255}
	applyEvent(winner, conflict.Apply)
	write := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	pending := func(want bool) {
		t.Helper()
		if err := fence.Wait(ctx); err != nil {
			t.Fatal(err)
		}
		_, _, found, err := conflict.NextTableRepair(ctx, db, "nb_cdc_source", "source_rows")
		if err != nil || found != want {
			t.Fatalf("repair pending=%t want=%t error=%v", found, want, err)
		}
	}
	write("UPDATE source_rows SET optional_text='losing-local',amount=0,raw_bytes=X'FF' WHERE source_id=?", key)
	pending(true)
	write("CREATE TRIGGER reject_repair_receipt BEFORE INSERT ON sync_apply_log FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned repair rollback'")
	if _, err := repair.RunOnce(ctx); err == nil {
		t.Fatal("repair receipt failure accepted")
	}
	if err := db.QueryRowContext(ctx, "SELECT optional_text FROM source_rows WHERE source_id=?", key).Scan(&got); err != nil || got != "losing-local" {
		t.Fatalf("failed repair was not rolled back: %s %v", got, err)
	}
	pending(true)
	write("DROP TRIGGER reject_repair_receipt")
	// Recreate the worker to prove repair intent is durable, not an in-memory callback.
	repair = apply.RepairWorker{DB: db, NodeID: "owned-fence", Rules: set, CaptureFence: fence}
	checkRepair := func() {
		t.Helper()
		if repaired, err := repair.RunOnce(ctx); err != nil || !repaired {
			t.Fatalf("repair=%t error=%v", repaired, err)
		}
		pending(false)
		var amount, raw string
		if err := db.QueryRowContext(ctx, "SELECT optional_text,CAST(amount AS CHAR),HEX(raw_bytes) FROM source_rows WHERE source_id=?", key).Scan(&got, &amount, &raw); err != nil || got != "repair-winner" || amount != "123456789012345678901234.123456" || raw != "0080FF" {
			t.Fatalf("winner image not restored: %s %s %s %v", got, amount, raw, err)
		}
	}
	checkRepair()
	write("DELETE FROM source_rows WHERE source_id=?", key)
	pending(true)
	checkRepair()
	write("UPDATE source_rows SET optional_text='after-own-repair' WHERE source_id=?", key)
	pending(true)
	checkRepair()
	applyEvent(makeEvent("owned-repair-tombstone", event.TypeDelete, "repair-winner", 2*time.Hour), conflict.Apply)
	write("INSERT INTO source_rows (source_id,amount,optional_text,raw_bytes,changed_at) VALUES (?,0,'stale-reinsert',X'01','2026-09-11 08:09:10.123456')", key)
	pending(true)
	if repaired, err := repair.RunOnce(ctx); err != nil || !repaired {
		t.Fatalf("tombstone repair=%t error=%v", repaired, err)
	}
	pending(false)
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM source_rows WHERE source_id=?", key).Scan(&count); err != nil || count != 0 {
		t.Fatalf("tombstone repair did not remove local loser: %d %v", count, err)
	}
	if repaired, err := repair.RunOnce(ctx); err != nil || repaired {
		t.Fatalf("completed repair repeated: %t %v", repaired, err)
	}
	t.Log("PASS: durable local-loser repair, receipt rollback+worker recreation, exact decimal/binary winner restoration, own UPDATE/INSERT/DELETE repair replay suppression, subsequent real local writes retained, tombstone repair and no repeated task")
	t.Log("PASS: checked SQL Apply with actual Canal fence/local registration; mapped incoming winner/duplicate/loser, receipt rollback+retry, tracked delete and no stale resurrection; production BIDIRECTIONAL gate remains closed")
}
