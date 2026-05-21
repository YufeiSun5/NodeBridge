package appconfig_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

func TestLoadEdgeExample(t *testing.T) {
	cfg, err := appconfig.LoadFile(filepath.Join("..", "..", "configs", "edge.example.yaml"))
	if err != nil {
		t.Fatalf("LoadFile edge example returned error: %v", err)
	}
	if cfg.Mode != appconfig.ModeEdge {
		t.Fatalf("expected mode edge, got %q", cfg.Mode)
	}
	if cfg.Node.ID != "edge-001" {
		t.Fatalf("expected node edge-001, got %q", cfg.Node.ID)
	}
}

func TestLoadServerExample(t *testing.T) {
	cfg, err := appconfig.LoadFile(filepath.Join("..", "..", "configs", "server.example.yaml"))
	if err != nil {
		t.Fatalf("LoadFile server example returned error: %v", err)
	}
	if cfg.Mode != appconfig.ModeServer {
		t.Fatalf("expected mode server, got %q", cfg.Mode)
	}
	if cfg.Node.ID != "server-001" {
		t.Fatalf("expected node server-001, got %q", cfg.Node.ID)
	}
}

func TestValidateRequiredFields(t *testing.T) {
	path := writeTempConfig(t, `
node:
  name: missing id
mysql:
  host: 127.0.0.1
`)

	_, err := appconfig.LoadFile(path)
	if err == nil {
		t.Fatal("expected validation error")
	}

	message := err.Error()
	for _, want := range []string{"mode is required", "node.id is required", "mysql.database is required"} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected error to contain %q, got %q", want, message)
		}
	}
}

func TestValidateInvalidMode(t *testing.T) {
	path := writeTempConfig(t, `
mode: desktop
node:
  id: edge-001
mysql:
  database: scada_edge
`)

	_, err := appconfig.LoadFile(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "mode must be edge or server") {
		t.Fatalf("expected invalid mode error, got %q", err.Error())
	}
}

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}
