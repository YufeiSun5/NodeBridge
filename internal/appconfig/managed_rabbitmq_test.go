package appconfig_test

import (
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

func TestNormalizeManagedRabbitMQUsesNodeIdentity(t *testing.T) {
	cfg := appconfig.Config{
		Mode: appconfig.ModeEdge,
		Node: appconfig.NodeConfig{ID: "edge-002"},
		RabbitMQ: appconfig.RabbitMQConfig{
			Mode: "managed", Install: true,
			LocalURL:  "amqp://old:old@127.0.0.1:5672/old",
			ServerURL: "amqp://old:old@192.168.10.10:5672/old",
		},
	}
	if err := appconfig.NormalizeManagedRabbitMQ(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.RabbitMQ.Username != "nb-edge-002-local" || cfg.RabbitMQ.Password != "1234" {
		t.Fatalf("unexpected local identity: %+v", cfg.RabbitMQ)
	}
	if !strings.Contains(cfg.RabbitMQ.LocalURL, "nb-edge-002-local:1234@127.0.0.1:5672/%2Fnodebridge-edge") {
		t.Fatalf("unexpected local URL %q", cfg.RabbitMQ.LocalURL)
	}
	if !strings.Contains(cfg.RabbitMQ.ServerURL, "nb-edge-002:1234@192.168.10.10:5672/%2Fnodebridge-server") {
		t.Fatalf("unexpected server URL %q", cfg.RabbitMQ.ServerURL)
	}
}

func TestNormalizeManagedRabbitMQServerIdentity(t *testing.T) {
	cfg := appconfig.Config{Mode: appconfig.ModeServer, Node: appconfig.NodeConfig{ID: "server-001"}, RabbitMQ: appconfig.RabbitMQConfig{Mode: "managed"}}
	if err := appconfig.NormalizeManagedRabbitMQ(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.RabbitMQ.Username != "nb-server-sync" || !strings.Contains(cfg.RabbitMQ.ServerURL, "nb-server-sync:1234@127.0.0.1:5672/%2Fnodebridge-server") {
		t.Fatalf("unexpected server identity: %+v", cfg.RabbitMQ)
	}
}

func TestManagedRabbitMQIdentityRejectsUnsafeNodeID(t *testing.T) {
	if _, err := appconfig.ManagedRabbitMQIdentityFor(appconfig.ModeEdge, "edge 002"); err == nil {
		t.Fatal("unsafe node id was accepted")
	}
}
