package rulecheck

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
)

func TestRulePreflightRealMySQL(t *testing.T) {
	raw := os.Getenv("NODEBRIDGE_BUSINESS_SAFETY_TEST_DSN")
	if raw == "" {
		t.Skip("NODEBRIDGE_BUSINESS_SAFETY_TEST_DSN required")
	}
	cfg, err := mysql.ParseDSN(raw)
	if err != nil {
		t.Fatal("invalid DSN")
	}
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil || cfg.Net != "tcp" || cfg.DBName != "" || (host != "127.0.0.1" && host != "::1") {
		t.Fatal("requires loopback and empty database")
	}
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 10 * time.Second
	cfg.WriteTimeout = 10 * time.Second
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	name := fmt.Sprintf("nb_preflight_%d", time.Now().UnixNano())
	user := fmt.Sprintf("nbpf_%d", time.Now().UnixNano())
	exec := func(query string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE DATABASE " + quoted(name))
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := db.ExecContext(cleanup, "DROP DATABASE "+quoted(name)); err != nil {
			t.Error(err)
		}
	}()
	table := quoted(name) + ".`items`"
	exec("CREATE TABLE " + table + " (id BIGINT PRIMARY KEY, code VARCHAR(64) UNIQUE, value INT NOT NULL) ENGINE=InnoDB")
	exec("INSERT INTO " + table + " VALUES (1,'unchanged',7)")
	exec("CREATE TABLE " + quoted(name) + ".`children` (id BIGINT PRIMARY KEY, parent_id BIGINT, FOREIGN KEY(parent_id) REFERENCES " + table + "(id)) ENGINE=InnoDB")
	exec("CREATE TRIGGER " + quoted(name) + ".`observed_trigger` BEFORE UPDATE ON " + table + " FOR EACH ROW SET NEW.value=NEW.value")
	rule := rules.SyncRule{ID: "owned", DatabaseName: name, TableName: "items", PrimaryKeys: []string{"id"}, Enable: true, Direction: rules.DirectionEdgeToServer, ConflictPolicy: rules.ConflictNone, DeleteMode: rules.DeleteHard}
	result, err := Check(ctx, db, rule, "target")
	if err != nil || !result.OK || len(result.CheckedPermissions) != 4 {
		t.Fatalf("admin check %+v err=%v", result, err)
	}
	for _, code := range []string{"table_triggers", "inbound_foreign_keys"} {
		found := false
		for _, finding := range result.Findings {
			found = found || finding.Code == code
		}
		if !found {
			t.Fatalf("missing dependency %s: %+v", code, result)
		}
	}
	var value int
	if err := db.QueryRowContext(ctx, "SELECT value FROM "+table+" WHERE id=1").Scan(&value); err != nil || value != 7 {
		t.Fatalf("preflight modified data: %d %v", value, err)
	}
	exec("CREATE USER '" + user + "'@'%' IDENTIFIED BY 'owned_test_only'")
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := db.ExecContext(cleanup, "DROP USER '"+user+"'@'%'"); err != nil {
			t.Error(err)
		}
	}()
	exec("GRANT SELECT ON " + table + " TO '" + user + "'@'%'")
	readCfg := *cfg
	readCfg.User = user
	readCfg.Passwd = "owned_test_only"
	reader, err := sql.Open("mysql", readCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := reader.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	result, err = Check(ctx, reader, rule, "target")
	if err != nil || result.OK {
		t.Fatalf("read-only account passed write checks: %+v %v", result, err)
	}
	for _, code := range []string{"permission_insert", "permission_update", "permission_delete"} {
		found := false
		for _, f := range result.Findings {
			if f.Code == code && strings.Contains(f.Message, "1142") {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s/1142: %+v", code, result)
		}
	}
	rule.DeleteMode = rules.DeleteSoft
	result, err = Check(ctx, db, rule, "target")
	if err != nil || result.OK {
		t.Fatalf("missing soft columns accepted: %+v %v", result, err)
	}
	rule.DeleteMode = rules.DeleteHard
	exec("DROP TABLE " + quoted(name) + ".`children`")
	exec("ALTER TABLE " + table + " DROP PRIMARY KEY, ADD PRIMARY KEY(id,code)")
	result, err = Check(ctx, db, rule, "target")
	if err != nil || result.OK {
		t.Fatalf("partial actual key accepted: %+v %v", result, err)
	}
	t.Log("isolated MySQL: connection success with DML denial detected; no writes; soft columns and actual composite key checked")
}
