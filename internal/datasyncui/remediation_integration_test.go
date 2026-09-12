package datasyncui

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
)

func TestRemediationMCPRealMySQL(t *testing.T) {
	raw := os.Getenv("NODEBRIDGE_BUSINESS_SAFETY_TEST_DSN")
	if raw == "" {
		t.Skip("NODEBRIDGE_BUSINESS_SAFETY_TEST_DSN required")
	}
	cfg, err := mysql.ParseDSN(raw)
	if err != nil {
		t.Fatal("invalid DSN")
	}
	host, portText, err := net.SplitHostPort(cfg.Addr)
	if err != nil || cfg.Net != "tcp" || cfg.DBName != "" || (host != "127.0.0.1" && host != "::1") {
		t.Fatal("requires loopback and empty database")
	}
	port, _ := strconv.Atoi(portText)
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := fmt.Sprintf("nb_mcp_fix_%d", time.Now().UnixNano())
	target := base + "_target"
	for _, name := range []string{base, target} {
		if _, err := db.ExecContext(ctx, "CREATE DATABASE `"+name+"`"); err != nil {
			t.Fatal(err)
		}
		defer func(name string) {
			cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := db.ExecContext(cleanup, "DROP DATABASE `"+name+"`"); err != nil {
				t.Error(err)
			}
		}(name)
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE `"+target+"`.items (id BIGINT PRIMARY KEY, value INT) ENGINE=InnoDB"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	rulesPath := filepath.Join(dir, "rules.yaml")
	service, err := NewMCPService(configPath, rulesPath, "", true)
	if err != nil {
		t.Fatal(err)
	}
	patch, _ := json.Marshal(map[string]any{"patch": map[string]any{"mode": "server", "node": map[string]any{"id": "owned-mcp"}, "mysql": map[string]any{"host": host, "port": port, "username": cfg.User, "password": cfg.Passwd, "database": base}}})
	if out := callMCP(t, service, "nodebridge_save_config_patch", string(patch)); strings.Contains(out, `"isError":true`) {
		t.Fatal("config setup failed")
	}
	original, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	rule := rules.SyncRule{ID: "owned", DatabaseName: "source_owned", TableName: "items", TargetDatabaseName: target, TargetTableName: "items", Direction: rules.DirectionEdgeToServer, ConflictPolicy: rules.ConflictNone, Enable: true, PrimaryKeys: []string{"id"}}
	args, _ := json.Marshal(map[string]any{"rules": []rules.SyncRule{rule}})
	if out := callMCP(t, service, "nodebridge_save_sync_rules", string(args)); !strings.Contains(out, "soft_delete_column_missing") {
		t.Fatalf("SOFT enabled without required columns: %s", out)
	}
	if _, err := os.Stat(rulesPath); !os.IsNotExist(err) {
		t.Fatal("rejected rules were persisted")
	}
	rule.DeleteMode = rules.DeleteHard
	args, _ = json.Marshal(map[string]any{"rules": []rules.SyncRule{rule}})
	if out := callMCP(t, service, "nodebridge_save_sync_rules", string(args)); strings.Contains(out, `"isError":true`) {
		t.Fatal(out)
	}
	if out := callMCP(t, service, "nodebridge_rule_preflight", `{"rule_id":"owned","side":"target"}`); strings.Contains(out, `"isError":true`) || !strings.Contains(out, `DELETE`) {
		t.Fatal(out)
	}
	if out := callMCP(t, service, "nodebridge_mysql_schema", `{"rule_id":"owned","side":"target","table":"items"}`); strings.Contains(out, `"isError":true`) || !strings.Contains(out, target) {
		t.Fatal(out)
	}
	mutation := `{"rule_id":"owned","side":"target","operation":"INSERT","table":"items","values":{"id":9007199254740993,"value":7},"expected_rows":1,"confirm":true}`
	if out := callMCP(t, service, "nodebridge_mysql_mutation_plan", mutation); strings.Contains(out, `"isError":true`) {
		t.Fatal(out)
	}
	if out := callMCP(t, service, "nodebridge_mysql_mutation_apply", mutation); strings.Contains(out, `"isError":true`) {
		t.Fatal(out)
	}
	if out := callMCP(t, service, "nodebridge_mysql_query", `{"rule_id":"owned","side":"target","table":"items","columns":["id","value"]}`); strings.Contains(out, `"isError":true`) || !strings.Contains(out, "9007199254740993") {
		t.Fatal(out)
	}
	if out := callMCP(t, service, "nodebridge_mysql_query", `{"rule_id":"owned","side":"target","table":"unrelated"}`); !strings.Contains(out, "table must match") {
		t.Fatal(out)
	}
	final, err := os.ReadFile(configPath)
	if err != nil || !bytes.Equal(original, final) {
		t.Fatal("rule-scoped operations changed default config")
	}
	t.Log("MCP stdio: SOFT activation rejected, HARD activation/preflight accepted, cross-rule schema/query/plan/apply verified without changing default database; large integer preserved")
}
