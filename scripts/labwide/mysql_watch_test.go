package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

const labStatementQuery = `SELECT t.PROCESSLIST_ID,s.EVENT_ID,s.EVENT_NAME,
CASE
 WHEN LOCATE('sync_upload_offset',COALESCE(s.SQL_TEXT,t.PROCESSLIST_INFO,''))>0 THEN 'sync_upload_offset'
 WHEN LOCATE('sync_apply_log',COALESCE(s.SQL_TEXT,t.PROCESSLIST_INFO,''))>0 THEN 'sync_apply_log'
 WHEN LOCATE('sync_event_log',COALESCE(s.SQL_TEXT,t.PROCESSLIST_INFO,''))>0 THEN 'sync_event_log'
 WHEN LOCATE('sync_ack_log',COALESCE(s.SQL_TEXT,t.PROCESSLIST_INFO,''))>0 THEN 'sync_ack_log'
 ELSE 'business_or_other' END,
COALESCE(w.EVENT_NAME,''),COALESCE(w.OPERATION,''),COALESCE(g.EVENT_NAME,'')
FROM performance_schema.threads t
JOIN performance_schema.events_statements_current s ON s.THREAD_ID=t.THREAD_ID
LEFT JOIN performance_schema.events_waits_current w ON w.THREAD_ID=t.THREAD_ID AND w.END_EVENT_ID IS NULL
LEFT JOIN performance_schema.events_stages_current g ON g.THREAD_ID=t.THREAD_ID AND g.END_EVENT_ID IS NULL
WHERE t.PROCESSLIST_USER='sync_user' AND t.PROCESSLIST_DB='scada_center'
AND s.END_EVENT_ID IS NULL AND t.PROCESSLIST_COMMAND<>'Sleep'`

type labStatementObservation struct {
	Connection uint64 `json:"connection"`
	Event      uint64 `json:"event"`
	Statement  string `json:"statement"`
	TableClass string `json:"table_class"`
	Wait       string `json:"wait"`
	Operation  string `json:"operation"`
	Stage      string `json:"stage"`
}

func observeLabStatements(ctx context.Context, db *sql.DB) ([]labStatementObservation, error) {
	rows, err := db.QueryContext(ctx, labStatementQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []labStatementObservation{}
	for rows.Next() {
		var row labStatementObservation
		if err := rows.Scan(&row.Connection, &row.Event, &row.Statement, &row.TableClass, &row.Wait, &row.Operation, &row.Stage); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// Opt-in, read-only observer. Neither SQL text nor parameter values are recorded.
func TestObserveLabMySQLStatements(t *testing.T) {
	dsn, statePath := os.Getenv("NODEBRIDGE_MYSQL_WATCH_DSN"), os.Getenv("NODEBRIDGE_MYSQL_WATCH_STATE")
	if dsn == "" || statePath == "" {
		t.Skip("NODEBRIDGE_MYSQL_WATCH_DSN and NODEBRIDGE_MYSQL_WATCH_STATE required")
	}
	statePath, err := filepath.Abs(statePath)
	if err != nil {
		t.Fatal(err)
	}
	runID := filepath.Base(filepath.Dir(statePath))
	if filepath.Base(statePath) != "state.json" || !strings.HasPrefix(runID, "widesmoke_") {
		t.Fatal("observer requires a smoke run state.json")
	}
	readState := func() (string, error) {
		data, err := os.ReadFile(statePath)
		if err != nil {
			return "", err
		}
		var state struct {
			RunID  string `json:"run_id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal(data, &state); err != nil {
			return "", err
		}
		if state.RunID != runID {
			return "", fmt.Errorf("observer run identity mismatch")
		}
		return state.Status, nil
	}
	status, err := readState()
	if err != nil || status != "running" {
		t.Fatalf("run is not active: status=%s error=%v", status, err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("invalid observer DSN")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	f, err := os.OpenFile(filepath.Join(filepath.Dir(statePath), "mysql-statements.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	deadline := time.Now().Add(35 * time.Minute)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	nextStateRead := time.Now()
	for time.Now().Before(deadline) {
		if time.Now().After(nextStateRead) {
			status, err = readState()
			if err != nil {
				t.Fatal(err)
			}
			if status == "completed" || status == "failed" || status == "stopped" {
				t.Logf("observer ended: run status=%s; this is not a workload verdict", status)
				return
			}
			nextStateRead = time.Now().Add(time.Second)
		}
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		observations, err := observeLabStatements(ctx, db)
		cancel()
		if err != nil {
			t.Fatalf("statement observation failed: %v", err)
		}
		if err := encoder.Encode(map[string]any{"at": start, "query_ms": float64(time.Since(start).Microseconds()) / 1000, "active": observations}); err != nil {
			t.Fatal(err)
		}
		<-ticker.C
	}
	t.Fatal("observer deadline exceeded")
}

func TestObserveLabStatementsRows(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			expect := mock.ExpectQuery("SELECT t.PROCESSLIST_ID")
			if fail {
				expect.WillReturnError(fmt.Errorf("permission denied"))
			} else {
				expect.WillReturnRows(sqlmock.NewRows([]string{"connection", "event", "statement", "class", "wait", "op", "stage"}).AddRow(3, 12, "statement/sql/insert", "sync_event_log", "wait/io/file/innodb/innodb_data_file", "read", "executing"))
			}
			rows, err := observeLabStatements(context.Background(), db)
			if fail && err == nil {
				t.Fatal("query failure was hidden")
			}
			if !fail && (err != nil || len(rows) != 1 || rows[0].Event != 12 || rows[0].TableClass != "sync_event_log") {
				t.Fatalf("rows=%+v err=%v", rows, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
