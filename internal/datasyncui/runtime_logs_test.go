package datasyncui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentlog"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

func TestMCPRuntimeLogNumericSecretPreservesJSONNumbers(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.yaml")
	cfg := validConfig()
	cfg.MySQL.Password = "1234"
	if err := appconfig.SaveFile(config, cfg); err != nil {
		t.Fatal(err)
	}
	s, err := NewMCPService(config, filepath.Join(dir, "rules.yaml"), "", true)
	if err != nil {
		t.Fatal(err)
	}
	writeRuntimeLogFixture(t, agentlog.Path(config), `{"time":"2026-09-10T01:02:05Z","level":"ERROR","msg":"failed credential 1234","pid":12345,"phase_ms":{"mysql_apply":1234.56}}`+"\n")
	value, err := s.Logs(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	line := value.(map[string]any)["items"].([]string)[0]
	var record map[string]any
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Fatalf("corrupt runtime JSON: %s", line)
	}
	if record["pid"] != float64(12345) || record["phase_ms"].(map[string]any)["mysql_apply"] != 1234.56 || strings.Contains(record["msg"].(string), "1234") {
		t.Fatalf("redaction=%s", line)
	}
	var response struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(callMCP(t, s, "nodebridge_logs", `{"limit":1}`)), &response); err != nil {
		t.Fatal(err)
	}
	var logs struct {
		Items []string `json:"items"`
	}
	if len(response.Result.Content) != 1 {
		t.Fatalf("response=%+v", response)
	}
	if err := json.Unmarshal([]byte(response.Result.Content[0].Text), &logs); err != nil {
		t.Fatal(err)
	}
	if len(logs.Items) != 1 {
		t.Fatalf("logs=%+v", logs)
	}
	if err := json.Unmarshal([]byte(logs.Items[0]), &record); err != nil {
		t.Fatalf("wire JSON corrupted: %s", logs.Items[0])
	}
	if record["pid"] != float64(12345) || record["phase_ms"].(map[string]any)["mysql_apply"] != 1234.56 {
		t.Fatalf("wire numbers changed: %+v", record)
	}
}

func writeRuntimeLogFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeLogsRealLevelTimeRotationAndSecrets(t *testing.T) {
	app, config, _ := newTempApp(t)
	cfg := validConfig()
	cfg.MySQL.Password = "private-test-password"
	app.config = &cfg
	path := agentlog.Path(config)
	writeRuntimeLogFixture(t, path+".1", `{"time":"2026-09-10T01:02:03Z","level":"ERROR","worker":"edge-downlink","msg":"older failure","event_id":"evt-old"}`+"\n")
	writeRuntimeLogFixture(t, path, "malformed\n"+`{"time":"2026-09-10T01:02:04Z","level":"WARN","worker":"server-cdc","msg":"slow step"}`+"\n"+`{"time":"2026-09-10T01:02:05Z","level":"ERROR","worker":"edge-downlink","msg":"apply failed","event_id":"evt-new","elapsed_ms":2500,"phase_ms":{"mysql_apply":2490},"error":"Error 1062 private-test-password"}`+"\n")
	logs := app.GetLogs(uiapi.LogQuery{Level: "error", Module: "edge-downlink", Limit: 2})
	if len(logs.Items) != 2 {
		t.Fatalf("logs=%+v", logs)
	}
	first := logs.Items[0]
	at, err := time.Parse(time.RFC3339Nano, first.Time)
	if err != nil || !at.Equal(time.Date(2026, 9, 10, 1, 2, 5, 0, time.UTC)) || first.Level != "ERROR" || first.Module != "edge-downlink" {
		t.Fatalf("entry=%+v err=%v", first, err)
	}
	for _, want := range []string{"evt-new", "elapsed_ms=2500", "mysql_apply", "1062"} {
		if !strings.Contains(first.Message, want) {
			t.Fatalf("missing %s in %+v", want, first)
		}
	}
	if strings.Contains(first.Message, cfg.MySQL.Password) || !strings.Contains(logs.Items[1].Message, "evt-old") {
		t.Fatalf("redaction/order=%+v", logs)
	}
	if logs := app.GetLogs(uiapi.LogQuery{Level: "WARN", Limit: 1}); len(logs.Items) != 1 || logs.Items[0].Module != "server-cdc" {
		t.Fatalf("warn=%+v", logs)
	}
}

func TestRuntimeLogsNotStarvedByMemory(t *testing.T) {
	app, config, _ := newTempApp(t)
	app.runtime.RecordProcessed("old-memory", "old", "apply", 1)
	writeRuntimeLogFixture(t, agentlog.Path(config), `{"time":"2099-01-01T00:00:00Z","level":"ERROR","worker":"edge-downlink","msg":"new persistent failure"}`+"\n")
	logs := app.GetLogs(uiapi.LogQuery{Limit: 1})
	if len(logs.Items) != 1 || logs.Items[0].Module != "edge-downlink" {
		t.Fatalf("logs=%+v", logs)
	}
}

func TestMCPRuntimeLogDiscoveryAndExplicitOverride(t *testing.T) {
	dir := t.TempDir()
	config, rules := filepath.Join(dir, "config.yaml"), filepath.Join(dir, "rules.yaml")
	s, err := NewMCPService(config, rules, "", true)
	if err != nil {
		t.Fatal(err)
	}
	writeRuntimeLogFixture(t, agentLogPath(config), "legacy-record\n")
	if out := callMCP(t, s, "nodebridge_logs", `{"limit":1}`); !strings.Contains(out, "legacy-record") {
		t.Fatal(out)
	}
	writeRuntimeLogFixture(t, agentlog.Path(config), `{"time":"2026-09-10T01:02:05Z","level":"ERROR","worker":"edge-downlink","msg":"runtime-record","event_id":"evt-visible"}`+"\n")
	writeRuntimeLogFixture(t, agentlog.Path(config)+".1", `{"time":"2026-09-10T01:02:04Z","level":"ERROR","worker":"edge-downlink","msg":"rotated-record"}`+"\n")
	if out := callMCP(t, s, "nodebridge_logs", `{"limit":2}`); !strings.Contains(out, "rotated-record") || !strings.Contains(out, "runtime-record") {
		t.Fatal(out)
	}
	if out := callMCP(t, s, "nodebridge_logs", `{"limit":1}`); !strings.Contains(out, "runtime-record") || !strings.Contains(out, "evt-visible") || strings.Contains(out, "legacy-record") {
		t.Fatal(out)
	}
	explicit, err := NewMCPService(config, rules, agentLogPath(config), true)
	if err != nil {
		t.Fatal(err)
	}
	if out := callMCP(t, explicit, "nodebridge_logs", `{"limit":1}`); !strings.Contains(out, "legacy-record") || strings.Contains(out, "runtime-record") {
		t.Fatal(out)
	}
}
