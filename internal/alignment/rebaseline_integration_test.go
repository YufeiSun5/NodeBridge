package alignment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/go-sql-driver/mysql"
)

func TestOwnedRebaselineTransactionRollback(t *testing.T) {
	if os.Getenv("NODEBRIDGE_OWNED_MULTI_FIXTURE") != "1" {
		t.Skip("owned isolated MySQL fixture required")
	}
	var endpoints []struct {
		Port int `json:"mysql_port"`
	}
	if json.Unmarshal([]byte(os.Getenv("NODEBRIDGE_MULTI_ENDPOINTS")), &endpoints) != nil || len(endpoints) != 3 || endpoints[1].Port < 1024 {
		t.Fatal("owned fixture port required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cfg := mysql.NewConfig()
	cfg.User, cfg.Passwd, cfg.Net, cfg.Addr = "root", "owned_multi_only", "tcp", "127.0.0.1:"+strconv.Itoa(endpoints[1].Port)
	cfg.ParseTime = true
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	database := fmt.Sprintf("nb_rebaseline_transaction_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if _, err := admin.ExecContext(cleanup, "DROP DATABASE "+database); err != nil {
			t.Error(err)
		}
	}()
	cfg.DBName = database
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	execute := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := mysqlconn.RunMigrations(ctx, db, "../../migrations/server"); err != nil {
		t.Fatal(err)
	}
	execute("CREATE TABLE target_rows(id BIGINT PRIMARY KEY,note VARCHAR(32),stamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB")
	execute("INSERT INTO target_rows(id,note) VALUES(99,'original')")
	schema, err := rulecheck.ReadSchema(ctx, db, database, "target_rows")
	if err != nil {
		t.Fatal(err)
	}
	p := RebaselinePlan{Mode: "server", Tables: []RebaselineTable{{Schema: schema, BackupTable: "backup_rows"}}}
	execute("INSERT INTO sync_rebaseline_generation(singleton,plan_json) VALUES(1,'{}')")
	execute("CREATE TRIGGER owned_fail_receipt BEFORE UPDATE ON sync_rebaseline_generation FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned receipt failure'")
	if err := prepareGenerationTables(ctx, db, p); err == nil || !strings.Contains(err.Error(), "owned receipt failure") {
		t.Fatal("receipt failure not exercised", err)
	}
	count := func(table string) int {
		t.Helper()
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count("target_rows") != 1 || count("backup_rows") != 0 {
		t.Fatal("backup and deletion did not roll back together")
	}
	execute("DROP TRIGGER owned_fail_receipt")
	if err := prepareGenerationTables(ctx, db, p); err != nil {
		t.Fatal(err)
	}
	if count("target_rows") != 0 || count("backup_rows") != 1 {
		t.Fatal("resume did not prepare")
	}
	var original, backup string
	if err := db.QueryRowContext(ctx, "SELECT CAST(stamp AS CHAR) FROM backup_rows WHERE id=99").Scan(&backup); err != nil {
		t.Fatal(err)
	}
	execute("INSERT INTO target_rows SELECT * FROM backup_rows")
	if err := prepareGenerationTables(ctx, db, p); err != nil {
		t.Fatal(err)
	}
	if count("target_rows") != 1 || count("backup_rows") != 1 {
		t.Fatal("retry erased restored baseline")
	}
	if err := db.QueryRowContext(ctx, "SELECT CAST(stamp AS CHAR) FROM target_rows WHERE id=99").Scan(&original); err != nil || original != backup {
		t.Fatal("default-generated timestamp was not preserved", err)
	}
	t.Log("PASS: receipt failure rolls back target deletion AND backup insertion; retry commits once and preserves all column values")
}
