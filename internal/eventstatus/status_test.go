package eventstatus

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRetryObservationsAndReceiptRecovery(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pid     int
		applied bool
		want    string
	}{
		{"live", 123, false, "retrying"}, {"stopped", 0, false, "retry_pending"}, {"other_process", 999, false, "retry_pending"}, {"applied", 123, true, "applied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "runtime.jsonl")
			line := `{"time":"2026-09-11T12:00:00Z","pid":123,"worker":"server-ingress","event_id":"evt","rule_id":"owned","target_database":"owned_db","target_table":"items","binlog_file":"mysql-bin.000001","binlog_pos":123,"msg":"worker step failed; retry scheduled","error":"secret target_row_missing","mysql_error_code":1142,"retry_after_ms":10000,"consecutive_errors":2}`
			if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			rows := sqlmock.NewRows([]string{"database_name", "table_name", "target_database_name", "target_table_name", "applied_at", "origin_node_id"})
			if tc.applied {
				rows.AddRow("source_db", "items", "owned_db", "items", time.Date(2026, 9, 11, 12, 0, 1, 0, time.UTC), "source")
			}
			mock.ExpectQuery("SELECT database_name").WithArgs("evt").WillReturnRows(rows)
			if tc.applied {
				mock.ExpectQuery("SELECT event_identity,decision").WillReturnRows(sqlmock.NewRows([]string{"event_identity", "decision"}))
			}
			result, err := (Service{DB: db, LogPath: path, RunningPID: tc.pid, Redact: func(v string) string { return strings.ReplaceAll(v, "secret", "[redacted]") }}).Query(context.Background(), Request{EventID: "evt"})
			if err != nil || len(result.Items) != 1 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			item := result.Items[0]
			if item.State != tc.want || item.RuleID != "owned" || item.BinlogPos != 123 || item.MySQLErrorCode != 1142 || strings.Contains(item.LastError, "secret") {
				t.Fatalf("item=%+v", item)
			}
			if !tc.applied && item.NextRetryAt != "2026-09-11T12:00:10Z" {
				t.Fatal(item.NextRetryAt)
			}
			if result.Complete {
				t.Fatal("bounded logs cannot be complete proof")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConflictReceiptOutcomeNeverClaimsLoserApplied(t *testing.T) {
	for _, outcome := range []string{"APPLY", "SUPERSEDED", "query-failed", "invalid"} {
		t.Run(outcome, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			at := time.Date(2026, 9, 11, 12, 0, 1, 0, time.UTC)
			mock.ExpectQuery("SELECT database_name").WithArgs("event").WillReturnRows(sqlmock.NewRows([]string{"database_name", "table_name", "target_database_name", "target_table_name", "applied_at", "origin_node_id"}).AddRow("source", "rows", "target", "rows", at, "node"))
			identity, _ := json.Marshal([]string{"node", "event"})
			hash := sha256.Sum256(identity)
			query := mock.ExpectQuery("SELECT event_identity,decision").WithArgs(hash[:])
			want := "applied"
			switch outcome {
			case "SUPERSEDED":
				want = "superseded"
				query.WillReturnRows(sqlmock.NewRows([]string{"identity", "decision"}).AddRow(identity, outcome))
			case "query-failed":
				want = "recorded_outcome_unknown"
				query.WillReturnError(context.DeadlineExceeded)
			case "invalid":
				want = "recorded_outcome_unknown"
				query.WillReturnRows(sqlmock.NewRows([]string{"identity", "decision"}).AddRow([]byte("wrong"), "APPLY"))
			default:
				query.WillReturnRows(sqlmock.NewRows([]string{"identity", "decision"}).AddRow(identity, outcome))
			}
			result, err := (Service{DB: db, LogPath: filepath.Join(t.TempDir(), "missing")}).Query(context.Background(), Request{EventID: "event"})
			if err != nil || len(result.Items) != 1 || result.Items[0].State != want || result.Items[0].ApplyStatus != want {
				t.Fatalf("%+v %v", result, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEmptyObservationsDoNotClaimHealthy(t *testing.T) {
	result, err := (Service{LogPath: filepath.Join(t.TempDir(), "missing")}).Query(context.Background(), Request{})
	if err != nil || len(result.Items) != 0 || len(result.Warnings) == 0 || result.Complete {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestReceiptFailureDoesNotClaimUnapplied(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT database_name").WithArgs("missing").WillReturnError(context.DeadlineExceeded)
	result, err := (Service{DB: db, LogPath: filepath.Join(t.TempDir(), "missing")}).Query(context.Background(), Request{EventID: "missing"})
	if err != nil || result.Items[0].ApplyStatus != "unknown" || result.Items[0].State != "unknown" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
