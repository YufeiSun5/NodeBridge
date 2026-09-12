package datasyncui

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

func TestContextToolPropagatesCancellationAndValidatesArguments(t *testing.T) {
	called := false
	tool := bindContextTool("diagnostics", "test", false, func(ctx context.Context, req struct {
		Limit int `json:"limit,omitempty"`
	}) (any, error) {
		called = true
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tool.call(ctx, json.RawMessage(`{}`)); err != context.Canceled || !called {
		t.Fatalf("called=%v err=%v", called, err)
	}
	called = false
	if _, err := tool.call(context.Background(), json.RawMessage(`{"sql":"SELECT secret"}`)); err == nil || called {
		t.Fatalf("unknown SQL argument accepted: %v", err)
	}
}

func TestMySQLDiagnosticsIsAvailableAndReadOnly(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.yaml")
	if err := appconfig.SaveFile(config, appconfig.Config{Mode: "edge", Node: appconfig.NodeConfig{ID: "edge"}, MySQL: appconfig.MySQLConfig{Database: "lab"}, MCP: appconfig.MCPServerConfig{Enable: true}}); err != nil {
		t.Fatal(err)
	}
	for _, lab := range []bool{false, true} {
		s, err := NewMCPService(config, filepath.Join(dir, "rules.yaml"), "", lab)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, tool := range s.Tools() {
			if tool["name"] == "nodebridge_mysql_diagnostics" {
				found = true
				if tool["annotations"].(map[string]any)["readOnlyHint"] != true {
					t.Fatal("diagnostic tool is not read-only")
				}
			}
		}
		if !found {
			t.Fatalf("lab=%v advertised=%v", lab, found)
		}
		if _, err := s.CallTool(context.Background(), "nodebridge_mysql_diagnostics", json.RawMessage(`{"sql":"SELECT secret"}`)); err == nil {
			t.Fatal("unknown/forbidden diagnostic request accepted")
		}
	}
}
