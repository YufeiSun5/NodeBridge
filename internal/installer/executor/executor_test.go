package executor_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	cfg.CDC.UseGTID = true
	cfg.MySQL.Password = `secret\value`
	confDir := filepath.Join(cfg.CDC.ConfigDir, "conf")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "canal.properties"), []byte("canal.destinations = example\ncanal.auto.scan = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacyDir := filepath.Join(cfg.CDC.ConfigDir, "nodebridge-edge-001")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "instance.properties"), []byte("legacy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "install-manifest.json")

	exec := executor.New()
	exec.Now = func() time.Time { return time.Unix(100, 0) }
	exec.InitRabbitMQ = func(context.Context, appconfig.Config) error {
		t.Fatal("rabbitmq init must be skipped when url is empty")
		return nil
	}
	exec.EnsureRabbitMQ = func(context.Context, appconfig.Config) error { return nil }

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
	configPath := filepath.Join(cfg.CDC.ConfigDir, "conf", "edge-001", "instance.properties")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read Canal destination config: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"canal.instance.gtidon=true",
		"canal.instance.dbUsername=sync",
		`canal.instance.dbPassword=secret\\value`,
		`canal.instance.filter.regex=scada\\..*`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("Canal destination config missing %q in %s", want, content)
		}
	}
	rootData, err := os.ReadFile(filepath.Join(confDir, "canal.properties"))
	if err != nil {
		t.Fatal(err)
	}
	rootContent := string(rootData)
	if !strings.Contains(rootContent, "canal.destinations = edge-001") || !strings.Contains(rootContent, "canal.auto.scan = false") {
		t.Fatalf("Canal root config did not activate destination: %s", rootContent)
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Fatalf("legacy Canal destination was not removed: %v", err)
	}
	if !loaded.OwnsCanalDestination("edge-001") {
		t.Fatalf("manifest does not own canonical Canal destination: %+v", loaded.ManagedComponents.Canal.Destinations)
	}
}

func TestApplyStopsOnRabbitMQInitError(t *testing.T) {
	cfg := testConfig()
	cfg.CDC.Mode = manifest.ModeExternal
	exec := executor.New()
	exec.SaveManifest = func(string, manifest.Manifest) error { return nil }
	exec.EnsureRabbitMQ = func(context.Context, appconfig.Config) error { return nil }
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
