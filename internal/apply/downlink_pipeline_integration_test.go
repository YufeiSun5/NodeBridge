package apply_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncruntime"
	"github.com/go-sql-driver/mysql"
)

func TestDownlinkBatchRollbackRequeueAndReplayReal(t *testing.T) {
	dsn, url := os.Getenv("NODEBRIDGE_APPLY_CONNECTION_TEST_DSN"), os.Getenv("NODEBRIDGE_APPLY_PIPELINE_AMQP_URL")
	if dsn == "" || url == "" {
		t.Skip("isolated MySQL and AMQP opt-in required")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid test DSN")
	}
	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("nb_downlink_rollback_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if _, err := admin.ExecContext(cleanup, "DROP DATABASE "+name); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}()
	cfg.DBName, cfg.InterpolateParams = name, true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := mysqlconn.RunMigrations(ctx, db, "../../migrations/edge"); err != nil {
		t.Fatal(err)
	}
	columns := []string{"id BIGINT PRIMARY KEY", "sync_version BIGINT NOT NULL", "updated_by_node VARCHAR(64)", "last_event_id VARCHAR(64)", "updated_at DATETIME(3)"}
	for i := 0; i < 45; i++ {
		columns = append(columns, fmt.Sprintf("f%02d VARCHAR(100)", i))
	}
	columns = append(columns, "CONSTRAINT fail_fixture CHECK (f00 <> 'blocked')")
	if _, err := db.ExecContext(ctx, "CREATE TABLE perf_stream ("+strings.Join(columns, ",")+") ENGINE=InnoDB"); err != nil {
		t.Fatal(err)
	}
	var events []mapper.MappedEvent
	for i := 1; i <= 3; i++ {
		events = append(events, connectionPerfEvent(name, "perf_stream", rules.SyncModeAppendOnly, event.TypeInsert, i, 1, fmt.Sprintf("rollback-%d", i)))
	}
	events[2].TargetAfter["f00"] = "blocked"
	t.Setenv("NODEBRIDGE_APPLY_PIPELINE_DIRECTION", "downlink")
	t.Setenv("NODEBRIDGE_APPLY_PIPELINE_LEGACY", "0")
	runtime := connectionPerfIngress(t, ctx, db, name, url, apply.NewSQLWorker(db), events).(*syncruntime.EdgeDownlinkBatchRuntime)
	runtime.Consumer.RequeueOnError = true
	source := runtime.Source.(syncruntime.AMQPBatchGetSource)
	assertCounts := func(wantRows, wantLogs, wantReady int) {
		t.Helper()
		for table, want := range map[string]int{"perf_stream": wantRows, "sync_apply_log": wantLogs} {
			var got int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&got); err != nil || got != want {
				t.Fatalf("%s=%d want=%d err=%v", table, got, want, err)
			}
		}
		queue, err := source.Channel.QueueDeclarePassive(source.Queue, true, false, false, false, nil)
		if err != nil || queue.Messages != wantReady {
			t.Fatalf("ready=%d want=%d err=%v", queue.Messages, wantReady, err)
		}
	}
	if _, err := runtime.RunOnce(ctx); err == nil {
		t.Fatal("expected check constraint failure")
	}
	assertCounts(0, 0, 3)
	if _, err := db.ExecContext(ctx, "ALTER TABLE perf_stream DROP CHECK fail_fixture"); err != nil {
		t.Fatal(err)
	}
	if result, err := runtime.RunOnce(ctx); err != nil || result.Count != 3 {
		t.Fatalf("retry=%+v err=%v", result, err)
	}
	assertCounts(3, 3, 0)
	replay := connectionPerfIngress(t, ctx, db, name, url, apply.NewSQLWorker(db), events)
	if result, err := replay.RunOnce(ctx); err != nil || result.Count != 3 {
		t.Fatalf("replay=%+v err=%v", result, err)
	}
	assertCounts(3, 3, 0)
}
