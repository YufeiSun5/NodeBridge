package mysqldiag

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func expectMetadata(m sqlmock.Sqlmock) {
	m.ExpectPing()
	m.ExpectQuery("SELECT @@innodb_buffer_pool_size").WillReturnRows(sqlmock.NewRows([]string{"bytes"}).AddRow(134217728))
	m.ExpectQuery("SELECT table_name, COALESCE").WithArgs("scada_center").WillReturnRows(sqlmock.NewRows([]string{"table", "rows", "data", "index"}).AddRow("sync_event_log", 77072, 800000000, 180000000))
	m.ExpectQuery("SELECT table_name,index_name").WithArgs("scada_center").WillReturnRows(sqlmock.NewRows([]string{"table", "index", "col", "pos", "prefix", "visible"}).AddRow("sync_event_log", "idx_table_status", "table_name", 1, nil, "YES").AddRow("sync_event_log", "idx_table_status", "status", 2, nil, "YES"))
}

func TestCollectMetadataAndExactCounts(t *testing.T) {
	db, m, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	expectMetadata(m)
	m.ExpectQuery("EXPLAIN FORMAT=JSON SELECT COUNT").WithArgs("orders", "orders_remote").WillReturnRows(sqlmock.NewRows([]string{"plan"}).AddRow(`{"query_block":{"table":{"key":"idx_table_status"}}}`))
	m.ExpectQuery("SELECT COUNT").WithArgs("orders", "orders_remote").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(120))
	m.ExpectQuery("SELECT COUNT.*status='FAILED'").WithArgs("orders", "orders_remote").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	r, err := Collect(context.Background(), db, "scada_center", Request{EventTables: []string{"orders", "orders_remote"}})
	if err != nil || r.Status != "ok" || len(r.Steps) != 7 {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	if r.Steps[6].Data.(map[string]int64)["count"] != 3 {
		t.Fatalf("failed events were hidden: %+v", r)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCollectDoesNotScanEventsByDefault(t *testing.T) {
	db, m, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	expectMetadata(m)
	r, err := Collect(context.Background(), db, "scada_center", Request{})
	if err != nil || r.Status != "ok" || len(r.Steps) != 4 {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCollectRetainsPartialResultsWithoutLeakingErrors(t *testing.T) {
	db, m, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	expectMetadata(m)
	m.ExpectQuery("EXPLAIN FORMAT=JSON").WithArgs("orders").WillReturnError(errors.New("private-password payload-secret"))
	m.ExpectQuery("SELECT COUNT").WithArgs("orders").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(20))
	m.ExpectQuery("SELECT COUNT.*status='FAILED'").WithArgs("orders").WillReturnError(errors.New("access denied"))
	r, err := Collect(context.Background(), db, "scada_center", Request{EventTables: []string{"orders"}})
	if err != nil || r.Status != "partial" || r.Steps[6].Data != nil || r.Steps[6].Status != "error" {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "private-password") || strings.Contains(string(b), "payload-secret") {
		t.Fatal("diagnostic error leaked private data")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCollectHonorsParentDeadline(t *testing.T) {
	db, m, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m.ExpectPing().WillDelayFor(time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r, err := Collect(ctx, db, "scada_center", Request{TimeoutSeconds: 10, EventTables: []string{"orders"}})
	if err != nil || r.Status != "partial" || r.ElapsedMS >= 500 {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	for i, step := range r.Steps {
		if step.ErrorCode != "deadline_exceeded" || (i > 0 && step.Status != "skipped") {
			t.Fatalf("step=%+v", step)
		}
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDiagnosticRequestBoundaries(t *testing.T) {
	for _, r := range []Request{{TimeoutSeconds: -1}, {TimeoutSeconds: 11}, {EventTables: []string{"orders;DROP TABLE x"}}, {EventTables: []string{"other_db.orders"}}, {EventTables: []string{"orders", "orders"}}, {EventTables: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}}} {
		if _, err := Collect(context.Background(), nil, "scada_center", r); err == nil {
			t.Fatalf("accepted %+v", r)
		}
	}
	if _, err := Collect(context.Background(), nil, "db;DROP", Request{}); err == nil {
		t.Fatal("invalid saved database accepted")
	}
}
