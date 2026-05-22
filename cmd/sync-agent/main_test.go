package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

func TestRunEdgeConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"-config", filepath.Join("..", "..", "configs", "edge.example.yaml")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v stderr=%s", err, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{"mode=edge", "node_id=edge-001"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, out)
		}
	}
}

func TestRunServerConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"-config", filepath.Join("..", "..", "configs", "server.example.yaml")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v stderr=%s", err, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{"mode=server", "node_id=server-001"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, out)
		}
	}
}

func TestRunMissingConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"-config", filepath.Join(t.TempDir(), "missing.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing config error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "load config failed") {
		t.Fatalf("expected stderr to mention load failure, got %q", stderr.String())
	}
}

func TestRunApplyEventRequiresFile(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"apply-event", "-config", filepath.Join("..", "..", "configs", "edge.example.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing file error")
	}
	if !strings.Contains(err.Error(), "file is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunPublishEventRequiresAMQPURL(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"publish-event", "-file", filepath.Join("..", "..", "sample-events", "device_config.update.json")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing amqp-url error")
	}
	if !strings.Contains(err.Error(), "amqp-url is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunPublishStressBatchRequiresAMQPURL(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"publish-stress-batch"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing amqp-url error")
	}
	if !strings.Contains(err.Error(), "amqp-url is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestBuildStressEventIsDeterministic(t *testing.T) {
	base := time.Date(2026, 5, 22, 9, 0, 0, 0, time.UTC)
	evt := buildStressEvent(stressEventOptions{
		Index:         7,
		PrimaryKeyID:  1007,
		EventIDPrefix: "evt-test",
		OriginNodeID:  "edge-003",
		DatabaseName:  "scada_edge",
		TableName:     "data_all",
		Now:           base,
	})

	if evt.EventID != "evt-test-00007" || evt.OriginNodeID != "edge-003" || evt.TableName != "data_all" {
		t.Fatalf("unexpected stress event %+v", evt)
	}
	if evt.PrimaryKey["id"] != 1007 || evt.After["value"] != "VALUE-00007" {
		t.Fatalf("unexpected stress event payload %+v", evt)
	}
	if evt.EventTime.Sub(evt.CreatedAt) != 7*time.Millisecond {
		t.Fatalf("unexpected event time offset %s", evt.EventTime.Sub(evt.CreatedAt))
	}
}

func TestRunPublishChangeOnceRequiresFile(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"publish-change-once", "-config", filepath.Join("..", "..", "configs", "edge.example.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing file error")
	}
	if !strings.Contains(err.Error(), "file is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunPublishChangeOnceRequiresAMQPURL(t *testing.T) {
	configPath := writeTempConfig(t, `
mode: edge
node:
  id: edge-test
  name: Edge Test
mysql:
  database: scada_edge
rabbitmq:
  server_url: amqp://127.0.0.1:5672/server-sync
sync:
  retry_interval_seconds: 1
log_web:
  enable: false
`)
	var stdout, stderr bytes.Buffer

	err := run([]string{
		"publish-change-once",
		"-config", configPath,
		"-rules", filepath.Join("..", "..", "configs", "sync-rules.example.yaml"),
		"-file", filepath.Join("..", "..", "sample-events", "device_config.change.json"),
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing amqp url error")
	}
	if !strings.Contains(err.Error(), "amqp-url or rabbitmq.local_url is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunCanalCheck(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"canal-check", "-config", filepath.Join("..", "..", "configs", "edge.example.yaml")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"reader=edge-001", "addr=127.0.0.1:11111", "destination=edge-001"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, out)
		}
	}
}

func TestRunCanalPublishOnceRequiresAMQPURL(t *testing.T) {
	configPath := writeTempConfig(t, `
mode: edge
node:
  id: edge-test
  name: Edge Test
mysql:
  database: scada_edge
rabbitmq:
  server_url: amqp://127.0.0.1:5672/server-sync
cdc:
  type: canal
  reader_name: edge-test
  canal_addr: 127.0.0.1:11111
  destination: edge-test
sync:
  retry_interval_seconds: 1
log_web:
  enable: false
`)
	var stdout, stderr bytes.Buffer

	err := run([]string{"canal-publish-once", "-config", configPath, "-rules", filepath.Join("..", "..", "configs", "sync-rules.example.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing amqp url error")
	}
	if !strings.Contains(err.Error(), "amqp-url or rabbitmq.local_url is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunConsumeOnceRequiresAMQPURL(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"consume-once", "-config", filepath.Join("..", "..", "configs", "server.example.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing amqp-url error")
	}
	if !strings.Contains(err.Error(), "amqp-url is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunConsumeOnceAcceptsEdgesFlagBeforeAMQPValidation(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"consume-once", "-config", filepath.Join("..", "..", "configs", "server.example.yaml"), "-edges", "edge-001,edge-002"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing amqp-url error")
	}
	if !strings.Contains(err.Error(), "amqp-url is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunConsumeDownlinkOnceRequiresAMQPURL(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"consume-downlink-once", "-config", filepath.Join("..", "..", "configs", "edge.example.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing amqp-url error")
	}
	if !strings.Contains(err.Error(), "amqp-url is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunRetryEventRequiresIDs(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"retry-event"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing event id error")
	}
	if !strings.Contains(err.Error(), "event-id is required") {
		t.Fatalf("unexpected error %v", err)
	}

	err = run([]string{"retry-event", "-event-id", "evt-001"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing target node id error")
	}
	if !strings.Contains(err.Error(), "target-node-id is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunRetryFailedBatchLoadsConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"retry-failed-batch", "-config", filepath.Join(t.TempDir(), "missing.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing config error")
	}
	if !strings.Contains(stderr.String(), "load config failed") {
		t.Fatalf("unexpected stderr %s err=%v", stderr.String(), err)
	}
}

func TestRunDeadLettersLoadsConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"dead-letters", "-config", filepath.Join(t.TempDir(), "missing.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing config error")
	}
	if !strings.Contains(stderr.String(), "load config failed") {
		t.Fatalf("unexpected stderr %s err=%v", stderr.String(), err)
	}
}

func TestRunManagedPlanOutputsOperations(t *testing.T) {
	configPath := writeTempConfig(t, `
mode: edge
node:
  id: edge-test
mysql:
  host: 127.0.0.1
  port: 3306
  username: sync
  database: scada_edge
rabbitmq:
  mode: external
  server_url: amqp://sync:secret@127.0.0.1:5672/server-sync
cdc:
  type: canal
  mode: external
  destination: edge-test
sync:
  retry_interval_seconds: 1
`)
	var stdout, stderr bytes.Buffer

	err := run([]string{"managed-plan", "-config", configPath, "-manifest", filepath.Join(t.TempDir(), "install-manifest.json")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("managed-plan returned error: %v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"mode": "plan"`) || !strings.Contains(out, "external-rabbitmq") {
		t.Fatalf("unexpected managed-plan output %s", out)
	}
}

func TestRunManagedApplyWritesManifestWithoutRabbitMQURL(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTempConfig(t, `
mode: edge
node:
  id: edge-test
mysql:
  host: 127.0.0.1
  port: 3306
  username: sync
  database: scada_edge
rabbitmq:
  mode: managed
  install: true
cdc:
  type: canal
  mode: managed
  install: true
  config_dir: `+filepath.ToSlash(filepath.Join(dir, "canal"))+`
  destination: edge-test
sync:
  retry_interval_seconds: 1
`)
	manifestPath := filepath.Join(dir, "install-manifest.json")
	var stdout, stderr bytes.Buffer

	err := run([]string{"managed-apply", "-config", configPath, "-manifest", manifestPath}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("managed-apply returned error: %v stderr=%s", err, stderr.String())
	}
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected manifest: %v", err)
	}
	if !strings.Contains(stdout.String(), `"mode": "apply"`) {
		t.Fatalf("unexpected managed-apply output %s", stdout.String())
	}
}

func TestRunInstallerAssetsCheckValidCatalog(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "otp.exe")
	content := []byte("fake installer")
	if err := os.WriteFile(assetPath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(dir, "catalog.json")
	catalog := fmt.Sprintf(`{
  "version": "test",
  "assets": [
    {
      "name": "otp",
      "component": "erlang",
      "path": %q,
      "sha256": "%x"
    }
  ]
}`, filepath.ToSlash(assetPath), sha256.Sum256(content))
	if err := os.WriteFile(catalogPath, []byte(catalog), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	err := run([]string{"installer-assets-check", "-catalog", catalogPath}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("installer-assets-check returned error: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"ok": true`) || !strings.Contains(stdout.String(), `"status": "ok"`) {
		t.Fatalf("unexpected installer-assets-check output %s", stdout.String())
	}
}

func TestRunInstallerCommandPlanOutputsPlannedOnly(t *testing.T) {
	catalogPath := filepath.Join(t.TempDir(), "catalog.json")
	catalog := `{
  "version": "test",
  "assets": [
    {
      "name": "rabbitmq",
      "component": "rabbitmq",
      "path": "C:\\packages\\rabbitmq.exe",
      "sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    }
  ]
}`
	if err := os.WriteFile(catalogPath, []byte(catalog), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	err := run([]string{"installer-command-plan", "-catalog", catalogPath}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("installer-command-plan returned error: %v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{`"action": "install"`, `"action": "service-install"`, `"status": "planned"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %s, got %s", want, out)
		}
	}
}

func TestRunReplayPendingOnceRequiresAMQPURL(t *testing.T) {
	configPath := writeTempConfig(t, `
mode: server
node:
  id: server-test
  name: Server Test
mysql:
  database: scada_center
rabbitmq: {}
sync:
  retry_interval_seconds: 1
log_web:
  enable: false
`)
	var stdout, stderr bytes.Buffer

	err := run([]string{"replay-pending-once", "-config", configPath}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing amqp url error")
	}
	if !strings.Contains(err.Error(), "amqp-url or rabbitmq.server_url is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunDispatchEventOnceRequiresFile(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"dispatch-event-once", "-config", filepath.Join("..", "..", "configs", "server.example.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing file error")
	}
	if !strings.Contains(err.Error(), "file is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunDispatchEventOnceRequiresAMQPURL(t *testing.T) {
	configPath := writeTempConfig(t, `
mode: server
node:
  id: server-test
  name: Server Test
mysql:
  database: scada_center
rabbitmq: {}
sync:
  retry_interval_seconds: 1
log_web:
  enable: false
`)
	var stdout, stderr bytes.Buffer

	err := run([]string{"dispatch-event-once", "-config", configPath, "-file", filepath.Join("..", "..", "sample-events", "device_config.insert.sync.json")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing amqp url error")
	}
	if !strings.Contains(err.Error(), "amqp-url or rabbitmq.server_url is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunServerCDCDispatchOnceRequiresFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"server-cdc-dispatch-once", "-config", filepath.Join("..", "..", "configs", "server.example.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing file error")
	}
	if !strings.Contains(err.Error(), "file is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunServerCanalDispatchOnceLoadsConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"server-canal-dispatch-once", "-config", filepath.Join(t.TempDir(), "missing.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing config error")
	}
	if !strings.Contains(err.Error(), "open") && !strings.Contains(stderr.String(), "load config failed") {
		t.Fatalf("unexpected error %v stderr=%s", err, stderr.String())
	}
}

func TestRunForwardUploadOnceRequiresURLs(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"forward-upload-once", "-local-amqp-url", "amqp://127.0.0.1"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing server-amqp-url error")
	}
	if !strings.Contains(err.Error(), "server-amqp-url is required") {
		t.Fatalf("unexpected error %v", err)
	}

	err = run([]string{"forward-upload-once", "-server-amqp-url", "amqp://127.0.0.1"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing local-amqp-url error")
	}
	if !strings.Contains(err.Error(), "local-amqp-url is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunServeLogWebRespectsDisabledConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"serve-log-web", "-config", filepath.Join("..", "..", "configs", "edge.example.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected disabled log web error")
	}
	if !strings.Contains(stderr.String(), "log web unavailable") {
		t.Fatalf("expected stderr to mention log web unavailable, got %q", stderr.String())
	}
}

func TestRunAgentEdgeRequiresLocalRabbitMQURL(t *testing.T) {
	configPath := writeTempConfig(t, `
mode: edge
node:
  id: edge-test
  name: Edge Test
mysql:
  database: scada_edge
rabbitmq:
  server_url: amqp://127.0.0.1:5672/server-sync
sync:
  retry_interval_seconds: 1
log_web:
  enable: false
`)
	var stdout, stderr bytes.Buffer

	err := run([]string{"run", "-config", configPath, "-rules", filepath.Join("..", "..", "configs", "sync-rules.example.yaml"), "-max-steps", "1"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing local_url error")
	}
	if !strings.Contains(err.Error(), "rabbitmq.local_url is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestRunAgentServerRequiresRabbitMQURL(t *testing.T) {
	configPath := writeTempConfig(t, `
mode: server
node:
  id: server-test
  name: Server Test
mysql:
  database: scada_center
rabbitmq: {}
sync:
  retry_interval_seconds: 1
log_web:
  enable: false
`)
	var stdout, stderr bytes.Buffer

	err := run([]string{"run", "-config", configPath, "-rules", filepath.Join("..", "..", "configs", "sync-rules.example.yaml"), "-max-steps", "1"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing server_url error")
	}
	if !strings.Contains(err.Error(), "rabbitmq.server_url is required") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestWatchStopFileCancelsContext(t *testing.T) {
	stopFile := filepath.Join(t.TempDir(), "sync-agent.stop")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout bytes.Buffer
	done := make(chan struct{})
	go func() {
		watchStopFile(ctx, stopFile, 5*time.Millisecond, cancel, &stdout)
		close(done)
	}()
	if err := os.WriteFile(stopFile, []byte("stop"), 0o600); err != nil {
		t.Fatalf("write stop file: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected stop file watcher to exit")
	}
	if ctx.Err() == nil {
		t.Fatal("expected context to be canceled")
	}
	if !strings.Contains(stdout.String(), "stop requested") {
		t.Fatalf("expected stop message, got %q", stdout.String())
	}
}

func TestWorkerConfigDefaultsRetryInterval(t *testing.T) {
	cfg := workerConfig("edge-upload", 0, 3)
	if cfg.Name != "edge-upload" || cfg.MaxSteps != 3 {
		t.Fatalf("unexpected config %+v", cfg)
	}
	if cfg.IdleInterval == 0 || cfg.ErrorInterval == 0 {
		t.Fatalf("expected default intervals, got %+v", cfg)
	}
}

func TestCanalConfigFromAppDefaultsReaderName(t *testing.T) {
	cfg := &appconfig.Config{
		Node: appconfig.NodeConfig{ID: "edge-001"},
		CDC: appconfig.CDCConfig{
			CanalAddr:   "127.0.0.1:11111",
			Destination: "edge-001",
			BatchSize:   500,
		},
	}
	canalConfig := canalConfigFromApp(cfg)
	if canalConfig.ReaderName != "edge-001" || canalConfig.Address != "127.0.0.1:11111" || canalConfig.BatchSize != 500 {
		t.Fatalf("unexpected canal config %+v", canalConfig)
	}
}

func TestTopologyForMode(t *testing.T) {
	serverTopology := topologyForMode("server", "edge-001, edge-002")
	if serverTopology.VHost != "/server-sync" {
		t.Fatalf("unexpected server topology %+v", serverTopology)
	}
	if len(serverTopology.Queues) < 4 {
		t.Fatalf("expected edge downlink queues, got %+v", serverTopology.Queues)
	}

	edgeTopology := topologyForMode("edge", "")
	if edgeTopology.VHost != "/edge-sync" {
		t.Fatalf("unexpected edge topology %+v", edgeTopology)
	}
}

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}
