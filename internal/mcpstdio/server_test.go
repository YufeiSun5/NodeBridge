package mcpstdio_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/mcpstdio"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestServerListsTools(t *testing.T) {
	var stdout bytes.Buffer
	server := mcpstdio.Server{Service: mcpstdio.StaticService{}}

	err := server.Serve(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"), &stdout)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(stdout.String(), "nodebridge_overview") || !strings.Contains(stdout.String(), `"id":1`) {
		t.Fatalf("unexpected tools/list response %s", stdout.String())
	}
}

func TestServerToolSchemasDescribeWriteInputs(t *testing.T) {
	var stdout bytes.Buffer
	server := mcpstdio.Server{Service: mcpstdio.StaticService{}}

	err := server.Serve(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"), &stdout)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"nodebridge_validate_config_patch", `"patch"`, `"rules"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in tools/list response %s", want, out)
		}
	}
}

func TestServerCallsOverviewReadOnly(t *testing.T) {
	var stdout bytes.Buffer
	service := mcpstdio.StaticService{
		ConfigPath: "config.yaml",
		Config: appconfig.Config{
			Mode:     appconfig.ModeServer,
			Node:     appconfig.NodeConfig{ID: "server-001"},
			RabbitMQ: appconfig.RabbitMQConfig{ServerURL: "amqp://sync:secret@127.0.0.1:5672/server-sync"},
		},
	}
	server := mcpstdio.Server{Service: service}

	input := `{"jsonrpc":"2.0","id":"call-1","method":"tools/call","params":{"name":"nodebridge_overview","arguments":{}}}` + "\n"
	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "server-001") || !strings.Contains(out, "mcp_read_only") {
		t.Fatalf("unexpected overview response %s", out)
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("overview response leaked secret: %s", out)
	}
}

func TestServerRejectsUnsupportedTool(t *testing.T) {
	var stdout bytes.Buffer
	server := mcpstdio.Server{Service: mcpstdio.StaticService{}}
	input := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"save_config","arguments":{}}}` + "\n"

	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(stdout.String(), "unsupported tool") {
		t.Fatalf("expected unsupported tool error, got %s", stdout.String())
	}
}

func TestServerIgnoresInitializedNotification(t *testing.T) {
	var stdout bytes.Buffer
	server := mcpstdio.Server{Service: mcpstdio.StaticService{}}
	input := `{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}` + "\n"

	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no response for notification, got %s", stdout.String())
	}
}

func TestServerReadsResources(t *testing.T) {
	var stdout bytes.Buffer
	service := mcpstdio.StaticService{
		ConfigPath: "config.yaml",
		RulesPath:  "rules.yaml",
		Config: appconfig.Config{
			Mode: appconfig.ModeEdge,
			Node: appconfig.NodeConfig{ID: "edge-001"},
		},
	}
	server := mcpstdio.Server{Service: service}
	input := `{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"nodebridge://overview"}}` + "\n"

	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(stdout.String(), "nodebridge://overview") || !strings.Contains(stdout.String(), "edge-001") {
		t.Fatalf("unexpected resources/read response %s", stdout.String())
	}
}

func TestServerReadsLogTailFromConfiguredFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "sync-agent.log")
	if err := os.WriteFile(logPath, []byte("first\nsecond\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	server := mcpstdio.Server{Service: mcpstdio.StaticService{LogPath: logPath}}
	input := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"nodebridge_logs","arguments":{"limit":10}}}` + "\n"

	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(stdout.String(), "first") || !strings.Contains(stdout.String(), "second") {
		t.Fatalf("expected log lines, got %s", stdout.String())
	}
}

func TestServerSavesNonSecretConfigPatch(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	auditPath := filepath.Join(dir, "mcp-audit.log")
	initial := appconfig.Config{
		Mode: appconfig.ModeEdge,
		Node: appconfig.NodeConfig{ID: "edge-001", Name: "old"},
		MySQL: appconfig.MySQLConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			Username: "sync",
			Password: "secret",
			Database: "scada_edge",
		},
		RabbitMQ: appconfig.RabbitMQConfig{ServerURL: "amqp://sync:secret@127.0.0.1:5672/server-sync"},
		Sync:     appconfig.SyncConfig{RetryIntervalSeconds: 10},
	}
	if err := appconfig.SaveFile(configPath, initial); err != nil {
		t.Fatal(err)
	}
	server := mcpstdio.Server{Service: mcpstdio.StaticService{
		ConfigPath: configPath,
		AuditPath:  auditPath,
		Config:     initial,
	}}
	input := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nodebridge_save_config_patch","arguments":{"patch":{"node":{"name":"remote edge"},"sync":{"apply_lanes":3},"mysql":{"host":"10.0.0.8"}}}}}` + "\n"
	var stdout bytes.Buffer

	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "saved") {
		t.Fatalf("expected saved response, got %s", out)
	}
	saved, err := appconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Node.Name != "remote edge" || saved.Sync.ApplyLanes != 3 || saved.MySQL.Host != "10.0.0.8" {
		t.Fatalf("patch was not persisted: %+v", saved)
	}
	if saved.MySQL.Password != "secret" || !strings.Contains(saved.RabbitMQ.ServerURL, "secret") {
		t.Fatalf("existing secrets were not preserved: %+v", saved)
	}
	audit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("expected audit log: %v", err)
	}
	if !strings.Contains(string(audit), "save_config_patch") {
		t.Fatalf("expected audit entry, got %s", string(audit))
	}
}

func TestServerValidatesConfigPatchWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	initial := appconfig.Config{
		Mode: appconfig.ModeEdge,
		Node: appconfig.NodeConfig{ID: "edge-001", Name: "old"},
		MySQL: appconfig.MySQLConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			Username: "sync",
			Password: "secret",
			Database: "scada_edge",
		},
		Sync: appconfig.SyncConfig{RetryIntervalSeconds: 10},
	}
	if err := appconfig.SaveFile(configPath, initial); err != nil {
		t.Fatal(err)
	}
	server := mcpstdio.Server{Service: mcpstdio.StaticService{ConfigPath: configPath}}
	input := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nodebridge_validate_config_patch","arguments":{"patch":{"node":{"name":"validated only"},"sync":{"apply_lanes":5}}}}}` + "\n"
	var stdout bytes.Buffer

	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "valid") || !strings.Contains(out, "validated only") {
		t.Fatalf("expected valid dry-run response, got %s", out)
	}
	saved, err := appconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Node.Name != "old" || saved.Sync.ApplyLanes != 0 {
		t.Fatalf("validate should not persist changes: %+v", saved)
	}
}

func TestServerRejectsSecretConfigPatch(t *testing.T) {
	server := mcpstdio.Server{Service: mcpstdio.StaticService{
		ConfigPath: filepath.Join(t.TempDir(), "config.yaml"),
		Config: appconfig.Config{
			Mode:  appconfig.ModeEdge,
			Node:  appconfig.NodeConfig{ID: "edge-001"},
			MySQL: appconfig.MySQLConfig{Database: "scada_edge"},
		},
	}}
	input := `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"nodebridge_save_config_patch","arguments":{"patch":{"mysql":{"password":"new-secret"}}}}}` + "\n"
	var stdout bytes.Buffer

	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(stdout.String(), "sensitive field") {
		t.Fatalf("expected sensitive field rejection, got %s", stdout.String())
	}
}

func TestServerAuditsRejectedSecretConfigPatch(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "mcp-audit.log")
	server := mcpstdio.Server{Service: mcpstdio.StaticService{
		ConfigPath: filepath.Join(dir, "config.yaml"),
		AuditPath:  auditPath,
		Config: appconfig.Config{
			Mode:  appconfig.ModeEdge,
			Node:  appconfig.NodeConfig{ID: "edge-001"},
			MySQL: appconfig.MySQLConfig{Database: "scada_edge"},
		},
	}}
	input := `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"nodebridge_save_config_patch","arguments":{"patch":{"mysql":{"password":"new-secret"}}}}}` + "\n"
	var stdout bytes.Buffer

	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(stdout.String(), "sensitive field") {
		t.Fatalf("expected sensitive field rejection, got %s", stdout.String())
	}
	audit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("expected reject audit log: %v", err)
	}
	if !strings.Contains(string(audit), "reject_config_patch") || strings.Contains(string(audit), "new-secret") {
		t.Fatalf("unexpected reject audit entry %s", string(audit))
	}
}

func TestServerSavesSyncRules(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "sync-rules.yaml")
	server := mcpstdio.Server{Service: mcpstdio.StaticService{RulesPath: rulesPath}}
	rule := rules.SyncRule{
		ID:             "device-config",
		DatabaseName:   "scada_edge",
		TableName:      "device_config",
		Direction:      rules.DirectionBidirectional,
		DispatchTarget: rules.DispatchActiveEdges,
		ConflictPolicy: rules.ConflictLastWriteWin,
		Enable:         true,
		PrimaryKeys:    []string{"id"},
	}
	input := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"nodebridge_save_sync_rules","arguments":{"rules":[` + ruleJSON(t, rule) + `]}}}` + "\n"
	var stdout bytes.Buffer

	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(stdout.String(), "rule_count") {
		t.Fatalf("expected rule_count response, got %s", stdout.String())
	}
	loaded, err := rules.LoadFile(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Rules) != 1 || loaded.Rules[0].ID != "device-config" {
		t.Fatalf("unexpected saved rules %+v", loaded.Rules)
	}
}

func TestServerRejectsInvalidSyncRulesAndAudits(t *testing.T) {
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "sync-rules.yaml")
	auditPath := filepath.Join(dir, "mcp-audit.log")
	server := mcpstdio.Server{Service: mcpstdio.StaticService{RulesPath: rulesPath, AuditPath: auditPath}}
	rule := rules.SyncRule{
		ID:             "bad-table",
		DatabaseName:   "scada_edge",
		TableName:      "bad-table",
		Direction:      rules.DirectionBidirectional,
		ConflictPolicy: rules.ConflictLastWriteWin,
		Enable:         true,
		PrimaryKeys:    []string{"id"},
	}
	input := `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"nodebridge_save_sync_rules","arguments":{"rules":[` + ruleJSON(t, rule) + `]}}}` + "\n"
	var stdout bytes.Buffer

	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(stdout.String(), "invalid identifier") {
		t.Fatalf("expected rule validation error, got %s", stdout.String())
	}
	if _, err := os.Stat(rulesPath); !os.IsNotExist(err) {
		t.Fatalf("invalid rules should not be saved, stat err=%v", err)
	}
	audit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("expected reject audit log: %v", err)
	}
	if !strings.Contains(string(audit), "reject_sync_rules") {
		t.Fatalf("expected reject audit entry, got %s", string(audit))
	}
}

func ruleJSON(t *testing.T, rule rules.SyncRule) string {
	t.Helper()
	data, err := json.Marshal(rule)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
