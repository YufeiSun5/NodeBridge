//go:build windows

package main

import (
	"context"
	"database/sql"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/cdc/canal"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
	"github.com/rabbitmq/amqp091-go"
)

func TestOwnedAlignmentCaptureProbe(t *testing.T) {
	if os.Getenv("NODEBRIDGE_OWNED_CDC_FIXTURE") != "1" {
		t.Skip("run scripts/test-canal-business-pipeline.ps1 -AlignmentCapture")
	}
	cfg, err := mysql.ParseDSN(os.Getenv("NODEBRIDGE_CDC_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil || cfg.Net != "tcp" || host != "127.0.0.1" || cfg.DBName != "" {
		t.Fatal("owned loopback database required")
	}
	address := os.Getenv("NODEBRIDGE_CDC_TEST_CANAL_ADDR")
	host, _, err = net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		t.Fatal("owned loopback Canal required")
	}
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 3*time.Second, 5*time.Second, 5*time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	for _, database := range []string{"nb_cdc_source", "nb_cdc_target"} {
		if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+database); err != nil {
			t.Fatal(err)
		}
		defer func() {
			cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
			defer done()
			if _, err := admin.ExecContext(cleanup, "DROP DATABASE "+database); err != nil {
				t.Error(err)
			}
		}()
	}
	cfg.DBName = "nb_cdc_source"
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := mysqlconn.RunMigrations(ctx, db, "../../migrations/edge"); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"CREATE TABLE nb_cdc_source.source_rows (id BIGINT PRIMARY KEY,note VARCHAR(20)) ENGINE=InnoDB", "CREATE TABLE nb_cdc_target.target_rows (id BIGINT PRIMARY KEY,note VARCHAR(20)) ENGINE=InnoDB"} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	left, err := alignment.Observe(ctx, db, "owned-probe", "nb_cdc_source", "source_rows")
	if err != nil {
		t.Fatal(err)
	}
	right, err := alignment.Observe(ctx, db, "owned-peer", "nb_cdc_target", "target_rows")
	if err != nil {
		t.Fatal(err)
	}
	rule := rules.SyncRule{ID: "probe", DatabaseName: "nb_cdc_source", TableName: "source_rows", TargetDatabaseName: "nb_cdc_target", TargetTableName: "target_rows", PrimaryKeys: []string{"id"}, DeleteMode: rules.DeleteHard, InitialAlignment: rules.InitialAlignment{Policy: rules.AlignmentManual}}
	plan, err := alignment.BuildPlan(rule, left, right, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	readerConfig := canal.Config{ReaderName: "owned-alignment-probe", Address: address, Destination: "example", Filter: `nb_cdc_source\.(source_rows|sync_capture_fence)`, BatchSize: 256}
	client, err := canal.NewWithlinClient(readerConfig)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := alignment.StartSnapshotCapture(ctx, db, client, "example", plan, rule, "owned-probe", true)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "INSERT INTO source_rows VALUES (1,'copied')"); err != nil {
		t.Fatal(err)
	}
	token, err := probe.WriteMarker(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO source_rows VALUES (2,'after-copy')"); err != nil {
		t.Fatal(err)
	}
	boundary, err := probe.WaitMarker(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if err := boundary.Validate(); err != nil {
		t.Fatal(err)
	}
	rolledBack, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rolledBack.Rollback()
	if _, err := rolledBack.ExecContext(ctx, "INSERT INTO source_rows VALUES (3,'rolled-back')"); err != nil {
		t.Fatal(err)
	}
	abortedToken, err := probe.WriteMarker(ctx, rolledBack)
	if err != nil {
		t.Fatal(err)
	}
	if err := rolledBack.Rollback(); err != nil {
		t.Fatal(err)
	}
	missing, stopMissing := context.WithTimeout(ctx, 300*time.Millisecond)
	_, err = probe.WaitMarker(missing, abortedToken)
	stopMissing()
	if err == nil {
		t.Fatal("rolled-back marker released capture boundary")
	}
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	// Reconnect with the identical Canal subscription. Every row inspected by
	// the probe must still be available because no probe read was acknowledged.
	replay, err := canal.NewWithlinClient(readerConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close(context.Background())
	if err := replay.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if err := replay.Subscribe(ctx, "example"); err != nil {
		t.Fatal(err)
	}
	found := map[string]canal.RowChange{}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && len(found) < 2 {
		rows, _, err := replay.Fetch(ctx, 256)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.DatabaseName == "nb_cdc_source" && row.TableName == "source_rows" && row.Operation == cdc.OperationInsert {
				note, _ := row.After["note"].(string)
				if note == "rolled-back" {
					t.Fatal("rolled-back business row captured")
				}
				found[note] = row
			}
		}
	}
	for _, item := range []struct {
		note   string
		before bool
	}{{"copied", true}, {"after-copy", false}} {
		row, ok := found[item.note]
		if !ok {
			t.Fatal("probe consumed business input", item.note)
		}
		before, err := boundary.BeforePosition(row.BinlogFile, row.BinlogPos)
		if err != nil || before != item.before {
			t.Fatalf("commit boundary %s: %v %v", item.note, before, err)
		}
		if row.BinlogFile == boundary.BinlogFile && row.BinlogPos == boundary.BinlogPos {
			t.Fatal("business row shares internal marker position")
		}
	}
	t.Log("PASS: Canal is live before snapshot work; committed transaction marker separates copy from later write; rollback cannot create a boundary; reconnect recovers all unacknowledged business rows")
	if err := replay.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	verifyCapturedSnapshotTransport(t, ctx, db, cfg, readerConfig, rule, false)
	// Reset only this test's owned databases for a second independent plan.
	for _, query := range []string{"DELETE FROM nb_cdc_source.source_rows WHERE id=4", "DELETE FROM nb_cdc_target.target_rows", "DELETE FROM nb_cdc_source.sync_alignment_job", "DELETE FROM nb_cdc_target.sync_alignment_job"} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	verifyCapturedSnapshotTransport(t, ctx, db, cfg, readerConfig, rule, true)
}

func verifyCapturedSnapshotTransport(t *testing.T, ctx context.Context, source *sql.DB, cfg *mysql.Config, sourceConfig canal.Config, rule rules.SyncRule, failBoundary bool) {
	t.Helper()
	cfg.DBName = "nb_cdc_target"
	target, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if err := mysqlconn.RunMigrations(ctx, target, "../../migrations/server"); err != nil {
		t.Fatal(err)
	}
	left, err := alignment.Observe(ctx, source, "owned-probe", "nb_cdc_source", "source_rows")
	if err != nil {
		t.Fatal(err)
	}
	right, err := alignment.Observe(ctx, target, "owned-peer", "nb_cdc_target", "target_rows")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := alignment.BuildPlan(rule, left, right, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	dbs := []*sql.DB{source, target}
	nodes := []string{"owned-probe", "owned-peer"}
	secondAddress := os.Getenv("NODEBRIDGE_CDC_TEST_SECOND_CANAL_ADDR")
	host, _, err := net.SplitHostPort(secondAddress)
	if err != nil || host != "127.0.0.1" {
		t.Fatal("second owned Canal required")
	}
	configs := []canal.Config{sourceConfig, {ReaderName: "owned-target-probe", Address: secondAddress, Destination: "example", Filter: `nb_cdc_target\.(target_rows|sync_capture_fence)`, BatchSize: 256}}
	probes := make([]*alignment.SnapshotCapture, 2)
	for i, db := range dbs {
		if _, err := alignment.PrepareSnapshotJob(ctx, db, plan, rule, nodes[i], true); err != nil {
			t.Fatal(err)
		}
		client, err := canal.NewWithlinClient(configs[i])
		if err != nil {
			t.Fatal(err)
		}
		probes[i], err = alignment.StartSnapshotCapture(ctx, db, client, "example", plan, rule, nodes[i], true)
		if err != nil {
			t.Fatal(err)
		}
		defer probes[i].Close()
	}
	broker := os.Getenv("NODEBRIDGE_CDC_TEST_RABBITMQ_URL")
	parsed, err := url.Parse(broker)
	if err != nil || parsed.Scheme != "amqp" || parsed.Hostname() != "127.0.0.1" {
		t.Fatal("owned loopback RabbitMQ required")
	}
	connections := make([]*amqp091.Connection, 2)
	for i := range connections {
		connections[i], err = amqp091.Dial(broker)
		if err != nil {
			t.Fatal(err)
		}
		defer connections[i].Close()
	}
	type completion struct {
		result alignment.CopyResult
		err    error
	}
	done := make(chan completion, 1)
	copyCtx, stop := context.WithTimeout(ctx, 15*time.Second)
	defer stop()
	if failBoundary {
		if _, err := target.ExecContext(ctx, "CREATE TRIGGER owned_boundary_failure BEFORE UPDATE ON sync_alignment_job FOR EACH ROW BEGIN IF NEW.capture_boundary IS NOT NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned boundary write failure'; END IF; END"); err != nil {
			t.Fatal(err)
		}
	}
	go func() {
		result, err := alignment.ReceiveCapturedSnapshotAMQP(copyCtx, target, connections[1], plan, rule, nodes[1], true, probes[1])
		done <- completion{result, err}
	}()
	sent, sendErr := alignment.SendCapturedSnapshotAMQP(copyCtx, source, connections[0], plan, rule, nodes[0], true, probes[0])
	if sendErr != nil {
		stop()
	}
	var received completion
	select {
	case received = <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("captured receiver did not finish")
	}
	if !failBoundary && (sendErr != nil || received.err != nil || received.result != sent || sent.Rows != 2) {
		t.Fatalf("captured transfer: %+v %v %+v", sent, sendErr, received)
	}
	if failBoundary {
		if sendErr == nil || received.err == nil || !strings.Contains(received.err.Error(), "1644") {
			t.Fatalf("post-commit failure not exercised: %v %v", sendErr, received.err)
		}
		job, err := alignment.ReadSnapshotJob(ctx, target, plan, nodes[1])
		if err != nil || job.Phase != alignment.JobTargetCommitted || job.Result == nil || job.CaptureMarker == "" || job.CaptureBoundary != nil {
			t.Fatal("post-commit outcome lost", job, err)
		}
		sourceJob, err := alignment.ReadSnapshotJob(ctx, source, plan, nodes[0])
		if err != nil || sourceJob.Phase != alignment.JobSourceReady || sourceJob.Result == nil || *sourceJob.Result != *job.Result {
			t.Fatal("source recovery manifest missing", sourceJob, err)
		}
		if _, err := alignment.ReconcileSnapshotReceipt(ctx, source, plan, rule, nodes[0], job, true); err == nil {
			t.Fatal("incomplete target capture accepted")
		}
		if _, err := target.ExecContext(ctx, "DROP TRIGGER owned_boundary_failure"); err != nil {
			t.Fatal(err)
		}
		if err := probes[1].Close(); err != nil {
			t.Fatal(err)
		}
		client, err := canal.NewWithlinClient(configs[1])
		if err != nil {
			t.Fatal(err)
		}
		probes[1], err = alignment.StartSnapshotRecoveryCapture(ctx, target, client, "example", plan, rule, nodes[1], true)
		if err != nil {
			t.Fatal(err)
		}
		defer probes[1].Close()
		recovered, err := alignment.RecoverTargetCapture(ctx, target, plan, rule, nodes[1], true, probes[1])
		if err != nil || recovered.CaptureBoundary == nil || recovered.CaptureMarker != job.CaptureMarker {
			t.Fatal("original committed marker recovery failed", recovered, err)
		}
		for attempt := 0; attempt < 2; attempt++ {
			if _, err := alignment.ReconcileSnapshotReceipt(ctx, source, plan, rule, nodes[0], recovered, true); err != nil {
				t.Fatal(err)
			}
		}
		sent = *recovered.Result
	}
	for i, db := range dbs {
		job, err := alignment.ReadSnapshotJob(ctx, db, plan, nodes[i])
		if err != nil {
			t.Fatal(err)
		}
		if job.CaptureBoundary == nil || job.CaptureMarker != job.CaptureBoundary.Token || job.Result == nil || *job.Result != sent {
			t.Fatal("copy committed without durable capture proof", job)
		}
		if err := alignment.CheckPendingJobs(ctx, db); err == nil {
			t.Fatal("boundary prematurely activated CDC")
		}
		postNote := "post-snapshot"
		if failBoundary {
			postNote = "post-recovery"
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO "+[]string{"source_rows", "target_rows"}[i]+" VALUES (4,?)", postNote); err != nil {
			t.Fatal(err)
		}
		if err := probes[i].Close(); err != nil {
			t.Fatal(err)
		}
		replay, err := canal.NewWithlinClient(configs[i])
		if err != nil {
			t.Fatal(err)
		}
		defer replay.Close(context.Background())
		if err := replay.Connect(ctx); err != nil {
			t.Fatal(err)
		}
		if err := replay.Subscribe(ctx, "example"); err != nil {
			t.Fatal(err)
		}
		seen := make(map[string]bool)
		until := time.Now().Add(5 * time.Second)
		for time.Now().Before(until) && len(seen) != 3 {
			rows, _, err := replay.Fetch(ctx, 256)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				if row.DatabaseName != cfg.DBName && i == 1 {
					continue
				}
				if row.TableName != []string{"source_rows", "target_rows"}[i] || row.Operation != cdc.OperationInsert {
					continue
				}
				note, _ := row.After["note"].(string)
				if note != "copied" && note != "after-copy" && note != postNote {
					continue
				}
				before, err := job.CaptureBoundary.BeforePosition(row.BinlogFile, row.BinlogPos)
				if err != nil || before != (note != postNote) {
					t.Fatalf("endpoint %d row %s boundary %v %v", i, note, before, err)
				}
				seen[note] = true
			}
		}
		if len(seen) != 3 {
			t.Fatal("captured copy consumed business backlog", i, seen)
		}
	}
	t.Log("PASS: RabbitMQ snapshot commits rows and target marker atomically; both endpoints persist observed Canal boundaries; restart retains copied and later rows on the correct sides of each boundary; durable activation fences remain")
}
