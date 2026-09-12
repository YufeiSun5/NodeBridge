package syncstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestReplayLogReadVisibilityExperiment(t *testing.T) {
	dsn := os.Getenv("NODEBRIDGE_REPLAY_VISIBILITY_DSN")
	if dsn == "" {
		t.Skip("set NODEBRIDGE_REPLAY_VISIBILITY_DSN for isolated MySQL visibility checks")
	}
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	config.DBName = ""
	admin, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	name := fmt.Sprintf("nodebridge_replay_it_%x", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"`"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, err := admin.ExecContext(cleanup, "DROP DATABASE `"+name+"`"); err != nil {
			t.Errorf("cleanup isolated replay database: %v", err)
		}
	})
	config.DBName = name
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "CREATE TABLE sync_apply_log (event_id VARCHAR(128) PRIMARY KEY) ENGINE=InnoDB"); err != nil {
		t.Fatal(err)
	}
	for _, outcome := range []string{"commit", "rollback", "cancel"} {
		t.Run(outcome, func(t *testing.T) {
			eventID := "visibility-" + outcome
			writer, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Rollback()
			if _, err := writer.ExecContext(ctx, "INSERT INTO sync_apply_log VALUES (?)", eventID); err != nil {
				t.Fatal(err)
			}
			var ordinary int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(1) FROM sync_apply_log WHERE event_id=?", eventID).Scan(&ordinary); err != nil || ordinary != 0 {
				t.Fatalf("ordinary pre-commit read: count=%d err=%v", ordinary, err)
			}
			reader, err := sql.Open("mysql", config.FormatDSN())
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			reader.SetMaxOpenConns(1)
			var connectionID int64
			if err := reader.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&connectionID); err != nil {
				t.Fatal(err)
			}
			lookupCtx, stopLookup := context.WithCancel(ctx)
			defer stopLookup()
			type result struct {
				count int
				err   error
			}
			done := make(chan result, 1)
			go func() {
				found, err := New(reader).Exists(lookupCtx, eventID)
				count := 0
				if found {
					count = 1
				}
				done <- result{count, err}
			}()
			// Observe the actual lock wait before releasing the writer.
			waiting := false
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				var waits int
				if err := admin.QueryRowContext(ctx, `SELECT COUNT(*) FROM performance_schema.data_lock_waits w JOIN performance_schema.threads t ON t.THREAD_ID=w.REQUESTING_THREAD_ID WHERE t.PROCESSLIST_ID=?`, connectionID).Scan(&waits); err != nil {
					t.Fatal(err)
				}
				if waits > 0 {
					waiting = true
					break
				}
				select {
				case got := <-done:
					t.Fatalf("locking read did not wait: %+v", got)
				default:
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !waiting {
				t.Fatal("locking read wait was not observed")
			}
			switch outcome {
			case "cancel":
				stopLookup()
			case "commit":
				err = writer.Commit()
			default:
				err = writer.Rollback()
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-done:
				if outcome == "cancel" {
					if !errors.Is(got.err, context.Canceled) || got.count != 0 {
						t.Fatalf("canceled store lookup: %+v", got)
					}
					t.Log("actual Store lookup canceled while waiting for writer")
					return
				}
				want := 0
				if outcome == "commit" {
					want = 1
				}
				if got.err != nil || got.count != want {
					t.Fatalf("locking read count=%d want=%d err=%v", got.count, want, got.err)
				}
				t.Logf("ordinary pre-commit count=0; observed locking wait; after writer completion count=%d", got.count)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}
