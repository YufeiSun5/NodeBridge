package mysqlconn

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/mysqldiag"
	"github.com/go-sql-driver/mysql"
)

func TestDiagnosticIndexRealMySQL(t *testing.T) {
	dsn := os.Getenv("NODEBRIDGE_MYSQL_DIAG_TEST_DSN")
	if dsn == "" {
		t.Skip("set NODEBRIDGE_MYSQL_DIAG_TEST_DSN for isolated real-MySQL verification")
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	name := fmt.Sprintf("nodebridge_diag_it_%x", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"`"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		if _, err := admin.ExecContext(cleanup, "DROP DATABASE `"+name+"`"); err != nil {
			t.Errorf("cleanup isolated diagnostic database %s: %v", name, err)
		}
	})
	config.DBName = name
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `CREATE TABLE sync_event_log (id BIGINT PRIMARY KEY, table_name VARCHAR(128) NOT NULL, pk_value VARCHAR(512) NOT NULL, status VARCHAR(32) NOT NULL, event_payload LONGTEXT, KEY idx_table_pk(table_name,pk_value)) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	count := 10000
	if raw := os.Getenv("NODEBRIDGE_MYSQL_DIAG_TEST_ROWS"); raw != "" {
		count, err = strconv.Atoi(raw)
		if err != nil || count < 1000 || count > 100000 {
			t.Fatal("integration rows must be 1000..100000")
		}
	}
	for first := 0; first < count; first += 500 {
		n := min(500, count-first)
		args := make([]any, 0, n*3)
		for i := first; i < first+n; i++ {
			state := "SUCCESS"
			if i%1000 == 0 {
				state = "FAILED"
			}
			args = append(args, i, strconv.Itoa(i), state)
		}
		query := "INSERT INTO sync_event_log VALUES " + strings.TrimSuffix(strings.Repeat("(?,'orders',?,?,REPEAT('x',4096)),", n), ",")
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, "ANALYZE TABLE sync_event_log"); err != nil {
		t.Fatal(err)
	}
	before, err := mysqldiag.Collect(ctx, db, name, mysqldiag.Request{EventTables: []string{"orders"}, TimeoutSeconds: 10})
	baselineTimeout := len(before.Steps) == 7 && before.Steps[6].Name == "failed_event_count" && before.Steps[6].Status == "error" && before.Steps[6].ErrorCode == "deadline_exceeded"
	if err != nil || (before.Status != "ok" && !baselineTimeout) {
		t.Fatalf("before status=%s err=%v", before.Status, err)
	}
	for _, step := range before.Steps[:6] {
		if step.Status != "ok" {
			t.Fatalf("unexpected baseline step failure: %s %s", step.Name, step.ErrorCode)
		}
	}
	if err := EnsureServerDiagnosticIndexes(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := EnsureServerDiagnosticIndexes(ctx, db); err != nil {
		t.Fatalf("repeat upgrade: %v", err)
	}
	after, err := mysqldiag.Collect(ctx, db, name, mysqldiag.Request{EventTables: []string{"orders"}, TimeoutSeconds: 10})
	if err != nil || after.Status != "ok" {
		t.Fatalf("after=%+v err=%v", after, err)
	}
	for _, r := range []mysqldiag.Report{before, after} {
		if r.Steps[5].Data.(map[string]int64)["count"] != int64(count) || (r.Steps[6].Status == "ok" && r.Steps[6].Data.(map[string]int64)["count"] != int64((count+999)/1000)) {
			t.Fatalf("count changed: %+v", r)
		}
	}
	plan := string(after.Steps[4].Data.(json.RawMessage))
	if !strings.Contains(plan, `"key": "idx_table_status"`) {
		t.Fatalf("covering index was not selected: %s", plan)
	}
	evidence := map[string]any{"passed": true, "fixture_rows": count, "payload_bytes_per_row": 4096, "before": before, "after": after, "baseline_failed_count_timed_out": baselineTimeout, "repeated_upgrade": true}
	evidence["apply_counts"] = verifyApplyIndexFixture(t, ctx, db, count)
	cliVerified := false
	if agent := os.Getenv("NODEBRIDGE_MYSQL_DIAG_TEST_AGENT"); agent != "" {
		cwd := t.TempDir()
		configPath := filepath.Join(cwd, "config.yaml")
		if err := os.WriteFile(configPath, []byte("mode: server\nnode:\n  id: diagnostic-it\nmysql:\n  database: "+name+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(ctx, agent, "migrate", "-config", configPath, "-scope", "server")
		command.Dir = cwd
		for _, value := range os.Environ() {
			if !strings.HasPrefix(strings.ToUpper(value), "NODEBRIDGE_SERVER_MYSQL_DSN=") {
				command.Env = append(command.Env, value)
			}
		}
		command.Env = append(command.Env, "NODEBRIDGE_SERVER_MYSQL_DSN="+config.FormatDSN())
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("packaged migrate from unrelated working directory failed: %v %s", err, output)
		}
		var systemTables int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=? AND table_name='sync_apply_log'", name).Scan(&systemTables); err != nil || systemTables != 1 {
			t.Fatalf("packaged schema was not applied: %d %v", systemTables, err)
		}
		for _, scope := range []string{"edge", "server"} {
			if _, err := db.ExecContext(ctx, "ALTER TABLE sync_apply_log DROP INDEX idx_table_op"); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(ctx, agent, "migrate", "-config", configPath, "-scope", scope)
			cmd.Dir, cmd.Env = cwd, append(append([]string{}, command.Env...), "NODEBRIDGE_EDGE_MYSQL_DSN="+config.FormatDSN())
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("packaged %s apply-index migration failed: %v %s", scope, err, output)
			}
			columns, err := diagnosticIndexColumns(ctx, db, "sync_apply_log", applyDiagnosticIndex)
			if err != nil || validateDiagnosticIndex(applyDiagnosticIndex, columns, []string{"table_name", "op_type"}) != nil {
				t.Fatalf("packaged %s apply index not restored: %v %v", scope, columns, err)
			}
		}
		cliVerified = true
	}
	evidence["packaged_cli_verified"] = cliVerified
	b, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("NODEBRIDGE_MYSQL_DIAG_TEST_REPORT"); path != "" {
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("isolated fixture %d rows: failed-count before %.3fms, after %.3fms; counts unchanged", count, before.Steps[6].ElapsedMS, after.Steps[6].ElapsedMS)
}

func verifyApplyIndexFixture(t *testing.T, ctx context.Context, db *sql.DB, count int) map[string]any {
	t.Helper()
	if _, err := db.ExecContext(ctx, `CREATE TABLE sync_apply_log (id BIGINT PRIMARY KEY, table_name VARCHAR(128) NOT NULL, pk_value VARCHAR(512) NOT NULL, op_type VARCHAR(16) NOT NULL, KEY idx_table_pk(table_name,pk_value)) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO sync_apply_log SELECT id, CONCAT('table_',MOD(id,4)), CONCAT(id,REPEAT('x',300)), ELT(1+MOD(id,3),'INSERT','UPDATE','DELETE') FROM sync_event_log`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "ANALYZE TABLE sync_apply_log"); err != nil {
		t.Fatal(err)
	}
	query := "SELECT table_name,op_type,COUNT(*) FROM sync_apply_log WHERE table_name IN ('table_0','table_1','table_2','table_3') GROUP BY table_name,op_type"
	collect := func() (map[string]int64, float64) {
		start := time.Now()
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		result := map[string]int64{}
		for rows.Next() {
			var table, op string
			var n int64
			if err := rows.Scan(&table, &op, &n); err != nil {
				t.Fatal(err)
			}
			result[table+"/"+op] = n
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return result, float64(time.Since(start).Microseconds()) / 1000
	}
	before, beforeMS := collect()
	for i := 0; i < 2; i++ {
		if err := EnsureApplyDiagnosticIndexes(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	after, afterMS := collect()
	var total int64
	for _, n := range after {
		total += n
	}
	if !reflect.DeepEqual(before, after) || total != int64(count) || len(after) != 12 {
		t.Fatalf("apply counts changed: before=%v after=%v total=%d", before, after, total)
	}
	var raw string
	if err := db.QueryRowContext(ctx, "EXPLAIN FORMAT=JSON "+query).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, `"key": "idx_table_op"`) || !strings.Contains(raw, `"using_index": true`) {
		t.Fatalf("apply covering index not selected: %s", raw)
	}
	t.Logf("apply operation counts: before %.3fms after %.3fms, %d rows unchanged", beforeMS, afterMS, total)
	return map[string]any{"passed": true, "before_ms": beforeMS, "after_ms": afterMS, "counts": after, "rows": total, "plan": json.RawMessage(raw), "repeated_upgrade": true}
}
