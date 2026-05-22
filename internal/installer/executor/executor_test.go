package executor_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/installer/executor"
	"github.com/YufeiSun5/NodeBridge/internal/installer/manifest"
)

func TestPlanExternalSkipsManagedChanges(t *testing.T) {
	cfg := testConfig()
	cfg.RabbitMQ.Mode = manifest.ModeExternal
	cfg.CDC.Mode = manifest.ModeExternal

	result := executor.New().Plan(executor.Request{Config: cfg, ManifestPath: "install-manifest.json"})

	requireOperation(t, result, "rabbitmq-rabbitmq", "noop")
	requireOperation(t, result, "canal-canal", "noop")
	if result.Manifest.ManagedComponents.RabbitMQ.Mode != manifest.ModeExternal ||
		result.Manifest.ManagedComponents.Canal.Mode != manifest.ModeExternal {
		t.Fatalf("expected external manifest, got %+v", result.Manifest)
	}
}

func TestApplyWritesManifestAndCanalConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()
	cfg.RabbitMQ.ServerURL = ""
	cfg.RabbitMQ.LocalURL = ""
	cfg.CDC.ConfigDir = filepath.Join(dir, "canal")
	manifestPath := filepath.Join(dir, "install-manifest.json")

	exec := executor.New()
	exec.Now = func() time.Time { return time.Unix(100, 0) }
	exec.InitRabbitMQ = func(context.Context, appconfig.Config) error {
		t.Fatal("rabbitmq init must be skipped when url is empty")
		return nil
	}

	result, err := exec.Apply(context.Background(), executor.Request{
		Config:       cfg,
		ManifestPath: manifestPath,
		Version:      "0.31.0",
		InstallID:    "install-test",
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	requireOperation(t, result, "manifest", "write")
	requireOperation(t, result, "canal-config", "write")

	loaded, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if loaded.InstallID != "install-test" || loaded.Version != "0.31.0" {
		t.Fatalf("unexpected manifest %+v", loaded)
	}
	configPath := filepath.Join(cfg.CDC.ConfigDir, "nodebridge-edge-001", "instance.properties")
	if _, err := filepath.Abs(configPath); err != nil {
		t.Fatalf("invalid config path: %v", err)
	}
}

func TestApplyStopsOnRabbitMQInitError(t *testing.T) {
	cfg := testConfig()
	cfg.CDC.Mode = manifest.ModeExternal
	exec := executor.New()
	exec.SaveManifest = func(string, manifest.Manifest) error { return nil }
	exec.InitRabbitMQ = func(context.Context, appconfig.Config) error { return errors.New("broker down") }

	result, err := exec.Apply(context.Background(), executor.Request{Config: cfg, ManifestPath: "manifest.json"})
	if err == nil {
		t.Fatal("expected apply error")
	}
	for _, operation := range result.Operations {
		if operation.Component == "rabbitmq-topology" && operation.Status == executor.StatusError {
			return
		}
	}
	t.Fatalf("expected rabbitmq topology error in %+v", result.Operations)
}

func requireOperation(t *testing.T, result executor.Result, component, action string) {
	t.Helper()
	for _, operation := range result.Operations {
		if operation.Component == component && operation.Action == action {
			return
		}
	}
	t.Fatalf("missing operation component=%s action=%s in %+v", component, action, result.Operations)
}

func testConfig() appconfig.Config {
	return appconfig.Config{
		Mode: appconfig.ModeServer,
		Node: appconfig.NodeConfig{ID: "edge-001"},
		MySQL: appconfig.MySQLConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			Username: "sync",
			Database: "scada_center",
		},
		RabbitMQ: appconfig.RabbitMQConfig{Mode: manifest.ModeManaged, Install: true, ServerURL: "amqp://127.0.0.1:5672/nodebridge"},
		CDC:      appconfig.CDCConfig{Type: "canal", Mode: manifest.ModeManaged, Install: true, Destination: "edge-001", Filter: "scada\\..*"},
		Sync:     appconfig.SyncConfig{RetryIntervalSeconds: 1},
	}
}
