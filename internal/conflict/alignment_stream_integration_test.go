package conflict_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func verifyStreamSnapshot(t *testing.T, ctx context.Context, db *sql.DB, database string) {
	t.Helper()
	for _, query := range []string{
		"CREATE TABLE stream_source (id BIGINT UNSIGNED PRIMARY KEY, amount DECIMAL(30,10) NOT NULL, payload VARBINARY(20), txt VARCHAR(30) NULL) ENGINE=InnoDB",
		"CREATE TABLE stream_target (target_id BIGINT UNSIGNED PRIMARY KEY, amount DECIMAL(30,10) NOT NULL, payload VARBINARY(20), txt VARCHAR(30) NULL) ENGINE=InnoDB",
		"INSERT INTO stream_source VALUES (1,12345678901234567890.1234567890,X'0080FF',NULL),(18446744073709551615,-1.1234567890,X'','')",
	} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	rule := rules.SyncRule{ID: "stream", DatabaseName: database, TableName: "stream_source", TargetDatabaseName: database, TargetTableName: "stream_target", PrimaryKeys: []string{"id"}, TargetPrimaryKeys: []string{"target_id"}, DeleteMode: rules.DeleteHard, InitialAlignment: rules.InitialAlignment{Policy: rules.AlignmentManual}}
	for _, mode := range []string{"success", "reverse", "lock_coverage", "out_of_order", "duplicate", "wrong_plan", "corrupt", "truncated", "fake_receipt", "lost_receipt", "sql_failure"} {
		t.Run("stream_"+mode, func(t *testing.T) {
			if _, err := db.ExecContext(ctx, "DELETE FROM stream_target"); err != nil {
				t.Fatal(err)
			}
			if mode == "reverse" {
				for _, query := range []string{"INSERT INTO stream_target SELECT * FROM stream_source", "DELETE FROM stream_source"} {
					if _, err := db.ExecContext(ctx, query); err != nil {
						t.Fatal(err)
					}
				}
			}
			left, err := alignment.Observe(ctx, db, "edge", database, "stream_source")
			if err != nil {
				t.Fatal(err)
			}
			right, err := alignment.Observe(ctx, db, "server", database, "stream_target")
			if err != nil {
				t.Fatal(err)
			}
			plan, err := alignment.BuildPlan(rule, left, right, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			receiver, err := alignment.NewSnapshotReceiver(ctx, db, plan, rule, true)
			if err != nil {
				t.Fatal(err)
			}
			defer receiver.Close()
			var first alignment.SnapshotFrame
			mustBlock := func(query string) {
				t.Helper()
				limited, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
				defer cancel()
				// A canceled autocommit may still execute after the lock is released.
				// Keep probes in uncommitted transactions so they cannot change data.
				tx, err := db.BeginTx(limited, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				_, err = tx.ExecContext(limited, query)
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("snapshot failed to hold writer lock: %s: %v", query, err)
				}
			}
			result, exportErr := alignment.ExportSnapshot(ctx, db, plan, rule, true, func(ctx context.Context, frame alignment.SnapshotFrame) (alignment.CopyResult, error) {
				b, err := alignment.EncodeSnapshotFrame(frame)
				if err != nil {
					return alignment.CopyResult{}, err
				}
				frame, err = alignment.DecodeSnapshotFrame(b)
				if err != nil {
					return alignment.CopyResult{}, err
				}
				if frame.Sequence == 0 {
					first = frame
					if mode == "lock_coverage" {
						for _, query := range []string{
							"UPDATE stream_source SET txt='blocked' WHERE id=1",
							"DELETE FROM stream_source WHERE id=1",
							"INSERT INTO stream_source VALUES (2,0,NULL,'blocked')",
							"INSERT INTO stream_target VALUES (2,0,NULL,'blocked')",
						} {
							mustBlock(query)
						}
					}
				}
				if frame.End != nil {
					if mode == "truncated" {
						return alignment.CopyResult{}, errors.New("owned_transport_disconnected")
					}
					if mode == "fake_receipt" {
						return alignment.CopyResult{}, nil
					}
				} else if frame.Sequence == 1 {
					switch mode {
					case "out_of_order":
						frame.Sequence++
					case "duplicate":
						frame = first
					case "wrong_plan":
						frame.PlanID = strings.Repeat("f", 64)
					case "corrupt":
						frame.Values[3] = []byte("changed")
					case "sql_failure":
						frame.Values[0] = []byte("1")
					}
				}
				receipt, err := receiver.Accept(ctx, frame)
				if err == nil && mode == "lock_coverage" {
					var visible int
					if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stream_target").Scan(&visible); err != nil {
						t.Fatal(err)
					}
					if frame.End == nil && visible != 0 {
						t.Fatal("uncommitted snapshot became visible")
					}
					if frame.End != nil {
						if visible != 2 {
							t.Fatal("target commit not visible with its receipt")
						}
						mustBlock("UPDATE stream_source SET txt='blocked' WHERE id=1")
					}
				}
				if err == nil && frame.End != nil && mode == "lost_receipt" {
					return alignment.CopyResult{}, errors.New("owned_commit_receipt_lost")
				}
				return receipt, err
			})
			receiver.Close()
			success := mode == "success" || mode == "reverse" || mode == "lock_coverage"
			if success && (exportErr != nil || result.Rows != 2) {
				t.Fatalf("copy result %+v: %v", result, exportErr)
			}
			if !success && exportErr == nil {
				t.Fatal("invalid stream succeeded")
			}
			if (mode == "lost_receipt" || mode == "fake_receipt" || mode == "truncated") && !strings.Contains(exportErr.Error(), "alignment_commit_unknown") {
				t.Fatalf("ambiguous commit treated as safe retry: %v", exportErr)
			}
			var count int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `"+plan.Target.Schema.Table+"`").Scan(&count); err != nil {
				t.Fatal(err)
			}
			want := 0
			if success || mode == "lost_receipt" {
				want = 2
			}
			if count != want {
				t.Fatalf("partial transfer leaked/committed copy lost: %d want %d", count, want)
			}
			if mode == "lock_coverage" {
				released, cancel := context.WithTimeout(ctx, time.Second)
				defer cancel()
				if _, err := db.ExecContext(released, "UPDATE stream_source SET txt=txt WHERE id=1"); err != nil {
					t.Fatal("source lock not released after receipt", err)
				}
			}
			if _, err := receiver.Accept(ctx, first); err == nil {
				t.Fatal("closed receiver reused")
			}
		})
	}
	t.Log("PASS: byte-framed mapped snapshot transfers both ways; disorder, duplicate, corruption, truncation and SQL failure rollback; lost commit receipt is ambiguous, not success")
}
