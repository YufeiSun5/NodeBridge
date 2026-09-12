package syncstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/go-sql-driver/mysql"
)

func TestEventLogBatchMatchesSequentialMySQL(t *testing.T) {
	dsn := os.Getenv("NODEBRIDGE_EVENT_LOG_BATCH_DSN")
	if dsn == "" {
		t.Skip("NODEBRIDGE_EVENT_LOG_BATCH_DSN required")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	cfg.DBName, cfg.ParseTime = "", true
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	name := fmt.Sprintf("nb_event_log_batch_%x", time.Now().UnixNano())
	var databases []*sql.DB
	for _, suffix := range []string{"_single", "_batch"} {
		schema := name + suffix
		if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+schema); err != nil {
			t.Fatal(err)
		}
		defer func() {
			cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
			defer done()
			if _, err := admin.ExecContext(cleanup, "DROP DATABASE "+schema); err != nil {
				t.Errorf("cleanup owned event log schema: %v", err)
			}
		}()
		copyCfg := *cfg
		copyCfg.DBName = schema
		db, err := sql.Open("mysql", copyCfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		db.SetMaxOpenConns(1)
		if _, err := db.ExecContext(ctx, "SET SESSION sql_mode='STRICT_ALL_TABLES'"); err != nil {
			t.Fatal(err)
		}
		if err := mysqlconn.RunMigrations(ctx, db, "../../migrations/server"); err != nil {
			t.Fatal(err)
		}
		databases = append(databases, db)
	}
	records := batchLogRecords(2*maxEventLogBatchRows + 4)
	for _, i := range []int{10, maxEventLogBatchRows + 1, len(records) - 1} {
		records[i].Event.EventID = records[0].Event.EventID
		records[i].Event.OriginNodeID = fmt.Sprintf("ignored-origin-%d", i)
		records[i].PKValue = fmt.Sprint(i)
		records[i].Status = StatusFailed
		records[i].ErrorMessage = fmt.Sprintf("error-%d", i)
		records[i].TargetTableName = fmt.Sprintf("target_%d", i)
	}
	records[len(records)-1].Status = StatusSuccess
	records[len(records)-1].SkipPayload = true
	records[len(records)-1].AppliedAt = fixedTime().Add(time.Minute)
	for _, i := range []int{20, 21, 22} {
		records[i].Payload, err = json.Marshal(map[string]any{"data": strings.Repeat("quote'\\\x00\n\u4e2d", 48000)})
		if err != nil {
			t.Fatal(err)
		}
	}
	sequential, batched := New(databases[0]), New(databases[1])
	sequential.Clock, batched.Clock = fixedTime, fixedTime
	for _, record := range records {
		if err := sequential.UpsertEventLog(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	if err := batched.UpsertEventLogs(ctx, records); err != nil {
		t.Fatal(err)
	}
	read := func(db *sql.DB) [][]sql.NullString {
		rows, err := db.QueryContext(ctx, `SELECT event_id,origin_node_id,source_node_id,database_name,table_name,
target_database_name,target_table_name,pk_value,op_type,direction,status,event_time,received_at,applied_at,error_message,event_payload
FROM sync_event_log ORDER BY event_id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var result [][]sql.NullString
		for rows.Next() {
			values := make([]sql.NullString, eventLogColumnCount)
			args := make([]any, len(values))
			for i := range values {
				args[i] = &values[i]
			}
			if err := rows.Scan(args...); err != nil {
				t.Fatal(err)
			}
			result = append(result, values)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return result
	}
	want, got := read(databases[0]), read(databases[1])
	if len(got) != len(records)-3 || !reflect.DeepEqual(got, want) {
		t.Fatal("batch differs from sequential upserts across duplicate IDs, metadata, timestamps or payloads")
	}
	rollback := batchLogRecords(maxEventLogBatchRows + 1)
	for i := range rollback {
		rollback[i].Event.EventID = fmt.Sprintf("rollback-%d", i)
	}
	rollback[len(rollback)-1].TargetTableName = strings.Repeat("x", 129)
	if err := batched.UpsertEventLogs(ctx, rollback); err == nil {
		t.Fatal("strict SQL failure accepted")
	}
	var count int
	if err := databases[1].QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_event_log WHERE event_id LIKE 'rollback-%'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("earlier chunk escaped rollback: count=%d error=%v", count, err)
	}
	t.Logf("verified all 16 fields for %d final events, duplicate order, oversized/special payloads and full rollback", len(got))
}
