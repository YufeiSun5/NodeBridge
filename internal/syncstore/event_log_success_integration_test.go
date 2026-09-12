package syncstore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestEventLogSuccessAvoidsInsertGapMySQL(t *testing.T) {
	dsn := os.Getenv("NODEBRIDGE_EVENT_LOG_BATCH_DSN")
	if dsn == "" {
		t.Skip("NODEBRIDGE_EVENT_LOG_BATCH_DSN required")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		t.Fatal("integration DSN must identify existing system schema")
	}
	source := "`" + strings.ReplaceAll(cfg.DBName, "`", "``") + "`.sync_event_log"
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	name := fmt.Sprintf("nb_event_success_%x", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if _, err := admin.ExecContext(cleanup, "DROP DATABASE "+name); err != nil {
			t.Error(err)
		}
	}()
	cfg.DBName, cfg.ParseTime = name, true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "CREATE TABLE sync_event_log LIKE "+source); err != nil {
		t.Fatal(err)
	}
	store := New(db)
	records := batchLogRecords(200)
	if err := store.UpsertEventLogs(ctx, records); err != nil {
		t.Fatal(err)
	}
	id := records[len(records)-1].Event.EventID
	at := fixedTime().Add(time.Minute)
	for trial, mode := range []string{"upsert", "status-update", "upsert"} {
		t.Run(fmt.Sprintf("%d-%s", trial, mode), func(t *testing.T) {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if mode == "upsert" {
				record := records[len(records)-1]
				record.Status, record.AppliedAt = StatusSuccess, at
				args, err := store.eventLogArgs(record)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := tx.ExecContext(ctx, eventLogUpsertSQL(), args...); err != nil {
					t.Fatal(err)
				}
			} else if err := markEventLogsSucceededTx(ctx, tx, []string{id}, at); err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			joined := false
			defer func() {
				_ = tx.Rollback()
				if !joined {
					<-result
				}
			}()
			go func() {
				writeCtx, done := context.WithTimeout(ctx, 3*time.Second)
				defer done()
				record := records[0]
				record.Event.EventID = fmt.Sprintf("concurrent-%d", trial)
				result <- store.UpsertEventLog(writeCtx, record)
			}()
			time.Sleep(200 * time.Millisecond)
			var locks int
			if err := admin.QueryRowContext(ctx, "SELECT COUNT(*) FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks r ON w.REQUESTING_ENGINE_LOCK_ID=r.ENGINE_LOCK_ID WHERE r.OBJECT_SCHEMA=?", name).Scan(&locks); err != nil {
				t.Fatal(err)
			}
			var writeErr error
			select {
			case writeErr = <-result:
				joined = true
			default:
			}
			blocked := !joined
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if !joined {
				writeErr = <-result
				joined = true
			}
			if writeErr != nil {
				t.Fatal(writeErr)
			}
			if mode == "upsert" && (!blocked || locks == 0) {
				t.Fatalf("baseline did not reproduce gap: blocked=%v locks=%d", blocked, locks)
			}
			if mode == "status-update" && (blocked || locks != 0) {
				t.Fatalf("status UPDATE blocked independent insert: blocked=%v locks=%d", blocked, locks)
			}
			t.Logf("mode=%s blocked=%v locks=%d", mode, blocked, locks)
		})
	}
	ids := make([]string, len(records))
	for i := range records {
		ids[i] = records[i].Event.EventID
	}
	if err := store.MarkEventLogsSucceeded(ctx, append(append([]string(nil), ids...), "missing"), at); err == nil {
		t.Fatal("missing log accepted")
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_event_log WHERE status='SUCCESS'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial completion escaped rollback: count=%d err=%v", count, err)
	}
	var payloadBefore string
	if err := db.QueryRowContext(ctx, "SELECT event_payload FROM sync_event_log WHERE event_id=?", id).Scan(&payloadBefore); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := store.MarkEventLogsSucceeded(ctx, append(append([]string(nil), ids...), ids[0]), at); err != nil {
			t.Fatal(err)
		}
	}
	var payloadAfter, status string
	if err := db.QueryRowContext(ctx, "SELECT event_payload,status FROM sync_event_log WHERE event_id=?", id).Scan(&payloadAfter, &status); err != nil {
		t.Fatal(err)
	}
	if payloadBefore != payloadAfter || status != StatusSuccess {
		t.Fatal("completion lost payload or status")
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_event_log WHERE status='SUCCESS'").Scan(&count); err != nil || count != len(records) {
		t.Fatalf("completion count=%d err=%v", count, err)
	}
}
