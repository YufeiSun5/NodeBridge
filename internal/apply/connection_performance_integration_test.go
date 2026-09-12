package apply_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"strconv"
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

// Opt-in; every trial owns a fresh database and keeps transaction durability unchanged.
func TestApplyConnectionPerformanceReal(t *testing.T) {
	dsn := os.Getenv("NODEBRIDGE_APPLY_CONNECTION_TEST_DSN")
	if dsn == "" {
		t.Skip("NODEBRIDGE_APPLY_CONNECTION_TEST_DSN required")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid test DSN")
	}
	cfg.DBName, cfg.ParseTime = "", true
	history := 0
	if raw := os.Getenv("NODEBRIDGE_APPLY_CONNECTION_HISTORY_ROWS"); raw != "" {
		history, err = strconv.Atoi(raw)
		if err != nil || history < 0 || history > 2000000 {
			t.Fatal("history rows must be 0..2000000")
		}
	}
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	for trial, interpolate := range []bool{false, true, true, false} {
		t.Run(fmt.Sprintf("trial_%d_interpolate_%t", trial, interpolate), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			name := fmt.Sprintf("nb_apply_conn_%d", time.Now().UnixNano())
			if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
				t.Fatal(err)
			}
			defer func() {
				cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
				defer done()
				if _, err := admin.ExecContext(cleanup, "DROP DATABASE "+name); err != nil {
					t.Errorf("cleanup owned database: %v", err)
				}
			}()
			trialCfg := *cfg
			trialCfg.DBName, trialCfg.InterpolateParams = name, interpolate
			db, err := sql.Open("mysql", trialCfg.FormatDSN())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := mysqlconn.RunMigrations(ctx, db, "../../migrations/server"); err != nil {
				t.Fatal(err)
			}
			seedConnectionHistory(t, ctx, db, history)
			columns := []string{"id BIGINT PRIMARY KEY", "sync_version BIGINT NOT NULL", "updated_by_node VARCHAR(64)", "last_event_id VARCHAR(64)", "updated_at DATETIME(3)"}
			for i := 0; i < 45; i++ {
				columns = append(columns, fmt.Sprintf("f%02d VARCHAR(100)", i))
			}
			for _, table := range []string{"perf_stream", "perf_state"} {
				if _, err := db.ExecContext(ctx, "CREATE TABLE "+table+" ("+strings.Join(columns, ",")+") ENGINE=InnoDB"); err != nil {
					t.Fatal(err)
				}
			}
			worker := apply.NewSQLWorker(db)
			var seed []mapper.MappedEvent
			for i := 1; i <= 200; i++ {
				seed = append(seed, connectionPerfEvent(name, "perf_state", rules.SyncModeOrderedCRUD, event.TypeInsert, i, 1, fmt.Sprintf("seed-%d", i)))
			}
			if _, err := worker.ApplyBatch(ctx, seed); err != nil {
				t.Fatal(err)
			}
			var events []mapper.MappedEvent
			for block := 0; block < 20; block++ {
				for i := 1; i <= 40; i++ {
					id := block*40 + i
					events = append(events, connectionPerfEvent(name, "perf_stream", rules.SyncModeAppendOnly, event.TypeInsert, id, 1, fmt.Sprintf("ins-%d", id)))
				}
				for i := 1; i <= 20; i++ {
					id := (block*20+i-1)%200 + 1
					events = append(events, connectionPerfEvent(name, "perf_state", rules.SyncModeOrderedCRUD, event.TypeUpdate, id, block/10+2, fmt.Sprintf("upd-%d-%d", block, i)))
				}
			}
			var ingress syncruntime.Stepper
			if url := os.Getenv("NODEBRIDGE_APPLY_PIPELINE_AMQP_URL"); url != "" {
				ingress = connectionPerfIngress(t, ctx, db, name, url, worker, events)
			}
			start := time.Now()
			for i := 0; i < len(events); i += 50 {
				end := min(i+50, len(events))
				if ingress != nil {
					result, err := ingress.RunOnce(ctx)
					if err != nil || result.Count != end-i {
						t.Fatalf("ingress chunk %d: count=%d err=%v", i, result.Count, err)
					}
					continue
				}
				result, err := worker.ApplyBatch(ctx, events[i:end])
				if err != nil || len(result.Results) != end-i {
					t.Fatalf("apply chunk %d: count=%d err=%v", i, len(result.Results), err)
				}
			}
			elapsed := time.Since(start)
			verifyConnectionValues(t, ctx, db, events)
			if ingress != nil && os.Getenv("NODEBRIDGE_APPLY_PIPELINE_DIRECTION") != "downlink" {
				var logged int
				if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_event_log WHERE status='SUCCESS'").Scan(&logged); err != nil || logged != len(events) {
					t.Fatalf("event logs=%d err=%v", logged, err)
				}
				t.Logf("pipeline includes durable queue GET, mapping, Apply, SUCCESS log and manual ACK")
			}
			var inserts, versions, logs int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM perf_stream").Scan(&inserts); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM perf_state WHERE sync_version=3").Scan(&versions); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_apply_log").Scan(&logs); err != nil {
				t.Fatal(err)
			}
			if inserts != 800 || versions != 200 || logs != 1400+history {
				t.Fatalf("counts inserts=%d version3=%d logs=%d", inserts, versions, logs)
			}
			t.Logf("direction=%s legacy=%s interpolate=%t history=%d events=1200 seconds=%.6f events_per_second=%.3f inserts=%d version3=%d logs=%d", os.Getenv("NODEBRIDGE_APPLY_PIPELINE_DIRECTION"), os.Getenv("NODEBRIDGE_APPLY_PIPELINE_LEGACY"), interpolate, history, elapsed.Seconds(), 1200/elapsed.Seconds(), inserts, versions, logs)
		})
	}
}

func connectionPerfEvent(database, table, mode, operation string, id, version int, eventID string) mapper.MappedEvent {
	hash := sha256.Sum256([]byte(eventID))
	eventID = fmt.Sprintf("%x", hash[:16])
	after := map[string]any{"id": id, "sync_version": version, "updated_by_node": "edge-perf", "last_event_id": "", "updated_at": "2026-09-09 12:00:00.000"}
	for i := 0; i < 45; i++ {
		after[fmt.Sprintf("f%02d", i)] = fmt.Sprintf("v%d-%d-'quoted'-%s", version, i, strings.Repeat("x", 24))
	}
	pk := map[string]any{"id": id}
	return mapper.MappedEvent{
		Event:          event.SyncEvent{EventID: eventID, OriginNodeID: "edge-perf", SourceNodeID: "edge-perf", TargetNodeID: "server-perf", EventType: operation, DatabaseName: database, TableName: table, PrimaryKey: pk, After: after},
		SourceDatabase: database, SourceTable: table, TargetDatabase: database, TargetTable: table,
		TargetPrimaryKey: pk, TargetAfter: after, SyncMode: mode,
	}
}

func seedConnectionHistory(t *testing.T, ctx context.Context, db *sql.DB, count int) {
	t.Helper()
	if count == 0 {
		return
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE perf_digits(n INT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO perf_digits VALUES (0),(1),(2),(3),(4),(5),(6),(7),(8),(9)"); err != nil {
		t.Fatal(err)
	}
	for base := 0; base < count; base += 1000 {
		_, err := db.ExecContext(ctx, `INSERT INTO sync_apply_log
(event_id,origin_node_id,source_node_id,target_node_id,database_name,table_name,target_database_name,target_table_name,pk_value,op_type,applied_at)
SELECT MD5(CONCAT('history-',?, '-',a.n+10*b.n+100*c.n)), 'edge-history','edge-history','server-history',
'source_history','nb_history_stream_table_with_realistic_name', 'target_history','nb_history_inbox_table_with_realistic_name',
CONCAT('{"id":',?+a.n+10*b.n+100*c.n,'}'),'INSERT',NOW(3)
FROM perf_digits a CROSS JOIN perf_digits b CROSS JOIN perf_digits c
WHERE a.n+10*b.n+100*c.n < ?`, base, base, min(1000, count-base))
		if err != nil {
			t.Fatal(err)
		}
	}
}
