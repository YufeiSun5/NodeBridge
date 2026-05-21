package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestWorkerConfigDefaultsRetryInterval(t *testing.T) {
	cfg := workerConfig("edge-upload", 0, 3)
	if cfg.Name != "edge-upload" || cfg.MaxSteps != 3 {
		t.Fatalf("unexpected config %+v", cfg)
	}
	if cfg.IdleInterval == 0 || cfg.ErrorInterval == 0 {
		t.Fatalf("expected default intervals, got %+v", cfg)
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
