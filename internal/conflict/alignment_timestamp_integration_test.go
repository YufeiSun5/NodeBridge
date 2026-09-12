package conflict_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
)

func verifyTimestampSnapshot(t *testing.T, ctx context.Context, database string) {
	t.Helper()
	open := func(zone string) *sql.DB {
		cfg, err := mysql.ParseDSN(os.Getenv("NODEBRIDGE_ALIGNMENT_TEST_DSN"))
		if err != nil {
			t.Fatal(err)
		}
		cfg.DBName = database
		cfg.InterpolateParams = true
		cfg.Params = map[string]string{"charset": "utf8mb4", "time_zone": "'" + zone + "'"}
		db, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		return db
	}
	source, target := open("+09:00"), open("-04:00")
	for _, item := range []struct {
		db    *sql.DB
		table string
	}{{source, "timestamp_source"}, {target, "timestamp_target"}} {
		if _, err := item.db.ExecContext(ctx, "CREATE TABLE "+item.table+" (id BIGINT PRIMARY KEY,ts TIMESTAMP(6) NULL) ENGINE=InnoDB"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := source.ExecContext(ctx, "INSERT INTO timestamp_source VALUES (1,'2026-09-11 10:20:30.123456'),(2,NULL)"); err != nil {
		t.Fatal(err)
	}
	left, err := alignment.Observe(ctx, source, "left", database, "timestamp_source")
	if err != nil {
		t.Fatal(err)
	}
	right, err := alignment.Observe(ctx, target, "right", database, "timestamp_target")
	if err != nil {
		t.Fatal(err)
	}
	rule := rules.SyncRule{ID: "timestamp", DatabaseName: database, TableName: "timestamp_source", TargetDatabaseName: database, TargetTableName: "timestamp_target", PrimaryKeys: []string{"id"}, DeleteMode: rules.DeleteHard, InitialAlignment: rules.InitialAlignment{Policy: rules.AlignmentManual}}
	plan, err := alignment.BuildPlan(rule, left, right, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	result, err := alignment.CopySnapshot(ctx, source, target, plan, rule, true)
	if err != nil || result.Rows != 2 {
		t.Fatalf("timestamp copy: %+v %v", result, err)
	}
	var sourceEpoch, targetEpoch, sourceZone, targetZone, display string
	if err := source.QueryRowContext(ctx, "SELECT CAST(UNIX_TIMESTAMP(ts) AS CHAR),@@session.time_zone FROM timestamp_source WHERE id=1").Scan(&sourceEpoch, &sourceZone); err != nil {
		t.Fatal(err)
	}
	if err := target.QueryRowContext(ctx, "SELECT CAST(UNIX_TIMESTAMP(ts) AS CHAR),@@session.time_zone,DATE_FORMAT(ts,'%Y-%m-%d %H:%i:%s.%f') FROM timestamp_target WHERE id=1").Scan(&targetEpoch, &targetZone, &display); err != nil {
		t.Fatal(err)
	}
	if sourceEpoch != targetEpoch || sourceZone != "+09:00" || targetZone != "-04:00" || display != "2026-09-10 21:20:30.123456" {
		t.Fatalf("timestamp/session drift: %s %s %s %s %s", sourceEpoch, targetEpoch, sourceZone, targetZone, display)
	}
	var isNull int
	if err := target.QueryRowContext(ctx, "SELECT ts IS NULL FROM timestamp_target WHERE id=2").Scan(&isNull); err != nil || isNull != 1 {
		t.Fatal("timestamp NULL changed", err)
	}
	t.Log("PASS: UTC snapshot preserves TIMESTAMP(6) instant and NULL across +09/-04 sessions without leaking session settings")
}
