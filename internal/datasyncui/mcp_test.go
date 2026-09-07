package datasyncui

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/mcpstdio"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

type fakeManagedRabbitMQAdmin struct {
	configs   []appconfig.Config
	edgeNodes []string
}

func (f *fakeManagedRabbitMQAdmin) EnsureForNode(_ context.Context, cfg appconfig.Config) error {
	f.configs = append(f.configs, cfg)
	return nil
}

func (f *fakeManagedRabbitMQAdmin) EnsureServerEdgeUser(_ context.Context, nodeID string) error {
	f.edgeNodes = append(f.edgeNodes, nodeID)
	return nil
}

func callMCP(t *testing.T, service *MCPService, name string, args string) string {
	t.Helper()
	var out bytes.Buffer
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}` + "\n"
	if err := (mcpstdio.Server{Service: service}).Serve(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestMCPLabBootstrapSecretsAndReload(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.yaml")
	s, err := NewMCPService(config, filepath.Join(dir, "rules.yaml"), "", true)
	if err != nil {
		t.Fatal(err)
	}
	patch := `{"patch":{"mode":"edge","node":{"id":"edge-lab"},"mysql":{"database":"lab","password":"first-secret"},"security":{"admin_password":"admin-secret"},"mcp_server":{"enable":false}}}`
	out := callMCP(t, s, "nodebridge_validate_config_patch", patch)
	if !strings.Contains(out, `valid`) || strings.Contains(out, "first-secret") {
		t.Fatal(out)
	}
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatal("dry run wrote config")
	}
	out = callMCP(t, s, "nodebridge_save_config_patch", patch)
	if !strings.Contains(out, `saved`) || strings.Contains(out, "first-secret") {
		t.Fatal(out)
	}
	cfg, err := appconfig.LoadFile(config)
	if err != nil || cfg.MySQL.Password != "first-secret" {
		t.Fatalf("save/decrypt failed: %v", err)
	}
	out = callMCP(t, s, "nodebridge_save_config_patch", `{"patch":{"mysql":{"password":"second-secret"},"rabbitmq":{"server_url":"amqp://lab:broker-secret@127.0.0.1:5672/lab"},"security":{"exit_password":"exit-secret"}}}`)
	if !strings.Contains(out, "saved") || strings.Contains(out, "broker-secret") {
		t.Fatal(out)
	}
	second, err := NewMCPService(config, filepath.Join(dir, "rules.yaml"), "", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewMCPService(config, filepath.Join(dir, "rules.yaml"), "", false); err == nil {
		t.Fatal("normal mode bypassed disabled MCP")
	}
	out = callMCP(t, second, "nodebridge_save_config_patch", `{"patch":{"mysql":{"password":"******"},"node":{"name":"reconnected"}}}`)
	if !strings.Contains(out, "saved") {
		t.Fatal(out)
	}
	cfg, err = appconfig.LoadFile(config)
	if err != nil || cfg.MySQL.Password != "second-secret" || cfg.Node.ID != "edge-lab" || cfg.Node.Name != "reconnected" {
		t.Fatalf("merge failed: %v", err)
	}
	out = callMCP(t, s, "nodebridge_save_sync_rules", `{}`)
	if !strings.Contains(out, `"isError":true`) {
		t.Fatal("missing rules was accepted: " + out)
	}
	out = callMCP(t, s, "nodebridge_save_config_patch", `{"patch":{"mysql":{"pasword":"typo"}}}`)
	if !strings.Contains(out, `"isError":true`) {
		t.Fatal("unknown field was accepted: " + out)
	}
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logs", "sync-agent.log"), []byte("password=second-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out = callMCP(t, s, "nodebridge_logs", `{"limit":1}`)
	if strings.Contains(out, "second-secret") {
		t.Fatal("log secret leaked")
	}
	audit, err := os.ReadFile(filepath.Join(dir, "logs", "mcp-audit.log"))
	if err != nil || strings.Contains(string(audit), "second-secret") {
		t.Fatal("audit missing or leaked secret")
	}
}

func TestMCPConcurrentPatchesPreserveFields(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.yaml")
	if err := appconfig.SaveFile(config, appconfig.Config{Mode: "edge", Node: appconfig.NodeConfig{ID: "edge"}, MySQL: appconfig.MySQLConfig{Database: "lab"}}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, patch := range []string{`{"patch":{"node":{"name":"concurrent"}}}`, `{"patch":{"node":{"location":"lab-two"}}}`} {
		s, err := NewMCPService(config, filepath.Join(dir, "rules.yaml"), "", true)
		if err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			out := callMCP(t, s, "nodebridge_save_config_patch", patch)
			if !strings.Contains(out, "saved") {
				t.Error(out)
			}
		}()
	}
	wg.Wait()
	cfg, err := appconfig.LoadFile(config)
	if err != nil || cfg.Node.Name != "concurrent" || cfg.Node.Location != "lab-two" {
		t.Fatalf("lost patch: %v", err)
	}
}

func TestMCPLabToolCatalog(t *testing.T) {
	dir := t.TempDir()
	s, err := NewMCPService(filepath.Join(dir, "config.yaml"), filepath.Join(dir, "rules.yaml"), "", true)
	if err != nil {
		t.Fatal(err)
	}
	tools := s.Tools()
	seen := map[string]bool{}
	for _, tool := range tools {
		name := tool["name"].(string)
		if seen[name] {
			t.Fatal("duplicate tool " + name)
		}
		seen[name] = true
	}
	for _, name := range []string{"nodebridge_start_agent", "nodebridge_stop_agent", "nodebridge_mysql_schema", "nodebridge_apply_managed_install", "nodebridge_ensure_server_edge_user"} {
		if !seen[name] {
			t.Fatal("missing " + name)
		}
	}
	data, _ := json.Marshal(tools)
	if !strings.Contains(string(data), `"admin_password"`) || !strings.Contains(string(data), `"mcp_server"`) {
		t.Fatal("full patch schema missing")
	}
	if !strings.Contains(string(data), `"restart_agent"`) {
		t.Fatal("save patch restart option missing")
	}
	out := callMCP(t, s, "nodebridge_start_agent", `{}`)
	if !strings.Contains(out, `"isError":true`) {
		t.Fatal("invalid startup did not fail: " + out)
	}
}

func TestMCPSaveConfigPatchCanRestartAgent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	cfg := validConfig()
	if err := appconfig.SaveFile(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	s, err := NewMCPService(configPath, filepath.Join(dir, "rules.yaml"), "", true)
	if err != nil {
		t.Fatal(err)
	}
	agent := newFakeAgentController()
	s.app.agent = agent

	out := callMCP(t, s, "nodebridge_save_config_patch", `{"patch":{"node":{"name":"restarted"}},"restart_agent":true}`)
	if !strings.Contains(out, `saved_and_restarted`) || agent.starts != 1 || !agent.running {
		t.Fatalf("save and restart failed: %s", out)
	}
	loaded, err := appconfig.LoadFile(configPath)
	if err != nil || loaded.Node.Name != "restarted" {
		t.Fatalf("config was not saved before restart: %v %+v", err, loaded)
	}
}

func TestMCPSaveConfigPatchMigratesManagedRabbitMQWithNodeID(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	cfg := validConfig()
	cfg.RabbitMQ = appconfig.RabbitMQConfig{
		Mode: "managed", Install: true,
		LocalURL:  "amqp://nb-edge-001-local:old@127.0.0.1:5672/%2Fnodebridge-edge",
		ServerURL: "amqp://nb-edge-001:old@192.168.10.10:5672/%2Fnodebridge-server",
		Username:  "nb-edge-001-local", Password: "old", VHost: "/nodebridge-edge",
	}
	if err := appconfig.SaveFile(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	s, err := NewMCPService(configPath, filepath.Join(dir, "rules.yaml"), "", true)
	if err != nil {
		t.Fatal(err)
	}
	admin := &fakeManagedRabbitMQAdmin{}
	s.rabbitMQAdmin = admin
	out := callMCP(t, s, "nodebridge_save_config_patch", `{"patch":{"node":{"id":"edge-002"}}}`)
	if !strings.Contains(out, "saved") || len(admin.configs) != 1 {
		t.Fatalf("managed save failed: %s", out)
	}
	loaded, err := appconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RabbitMQ.Username != "nb-edge-002-local" || loaded.RabbitMQ.Password != "1234" || !strings.Contains(loaded.RabbitMQ.ServerURL, "nb-edge-002:1234@") {
		t.Fatalf("managed RabbitMQ config was not migrated: %+v", loaded.RabbitMQ)
	}
}

func TestMCPServerCanProvisionEdgeAccount(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	cfg := validConfig()
	cfg.Mode = appconfig.ModeServer
	cfg.RabbitMQ = appconfig.RabbitMQConfig{Mode: "managed", Install: true}
	if err := appconfig.SaveFile(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	s, err := NewMCPService(configPath, filepath.Join(dir, "rules.yaml"), "", true)
	if err != nil {
		t.Fatal(err)
	}
	admin := &fakeManagedRabbitMQAdmin{}
	s.rabbitMQAdmin = admin
	out := callMCP(t, s, "nodebridge_ensure_server_edge_user", `{"node_id":"edge-002"}`)
	if !strings.Contains(out, "nb-edge-002") || len(admin.edgeNodes) != 1 || admin.edgeNodes[0] != "edge-002" {
		t.Fatalf("server edge account was not provisioned: %s", out)
	}
}

func TestMCPRequiredMutationArgument(t *testing.T) {
	called := false
	tool := bindTool("startup", "", true, func(req uiapi.SetAutoStartRequest) (any, error) { called = true; return req, nil })
	if _, err := tool.call(json.RawMessage(`{}`)); err == nil || called {
		t.Fatal("missing enabled changed startup")
	}
	if _, err := tool.call(json.RawMessage(`{"enabled":false}`)); err != nil || !called {
		t.Fatal("explicit false was rejected")
	}
}

func TestAgentDiscoveryAndStopAcrossControllers(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.yaml")
	stopFile := filepath.Join(dir, "run", "agent.stop")
	release, err := agentstate.Acquire(config, stopFile)
	if err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	cleanup := func() { once.Do(release) }
	t.Cleanup(cleanup)
	controller := newExternalAgentController()
	controller.configPath = config
	if !controller.Running() || controller.Status().PID != os.Getpid() {
		t.Fatal("other controller did not discover agent")
	}
	if err := controller.Start(context.Background(), config, "rules", stopFile); err != errAgentAlreadyRunning {
		t.Fatalf("duplicate start: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := os.Stat(stopFile); err == nil {
					cleanup()
					return
				}
			}
		}
	}()
	state, err := controller.Stop(ctx, stopFile, time.Second)
	<-done
	if err != nil || state != "stopped" {
		t.Fatalf("remote stop: %s %v", state, err)
	}
}
