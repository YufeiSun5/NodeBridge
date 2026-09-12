package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

func TestAgentStartupFailurePersistsDiagnostic(t *testing.T) {
	path := writeTempConfig(t, "mode: server\nnode:\n  id: server-log-test\nmysql:\n  database: test\nrabbitmq: {}\n")
	var stdout, stderr bytes.Buffer
	err := runAgent([]string{"-config", path, "-rules", filepath.Join("..", "..", "configs", "sync-rules.example.yaml"), "-max-steps", "1"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected unavailable broker config")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(path), "logs", "sync-runtime.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		if record["msg"] == "agent run failed" {
			found = true
			if record["node_id"] != "server-log-test" || record["version"] == nil || record["pid"] == nil || record["level"] != "ERROR" {
				t.Fatalf("incomplete diagnostic %s", line)
			}
		}
	}
	if !found {
		t.Fatalf("failure not persisted: %s", data)
	}
}

func TestRuntimeRedactorIncludesURLAndConfigCredentials(t *testing.T) {
	cfg := &appconfig.Config{}
	cfg.MySQL.Password = "mysql-secret"
	cfg.RabbitMQ.ServerURL = "amqp://user:url-secret@host/vhost"
	cfg.Security.AdminPassword = "admin-secret"
	got := runtimeRedactor(cfg)("mysql-secret url-secret admin-secret")
	for _, secret := range []string{"mysql-secret", "url-secret", "admin-secret"} {
		if strings.Contains(got, secret) {
			t.Fatal("secret retained")
		}
	}
}
