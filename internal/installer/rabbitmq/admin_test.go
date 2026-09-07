package rabbitmq_test

import (
	"context"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	rabbitadmin "github.com/YufeiSun5/NodeBridge/internal/installer/rabbitmq"
)

func TestAdminCreatesNodeSpecificLocalUserAndChangesPassword(t *testing.T) {
	var calls []string
	admin := rabbitadmin.Admin{CLI: "rabbitmqctl.bat", Runner: func(_ context.Context, _ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		switch args[0] {
		case "list_vhosts":
			return "/\n", nil
		case "list_users":
			return "guest [administrator]\n", nil
		default:
			return "", nil
		}
	}}
	cfg := appconfig.Config{Mode: appconfig.ModeEdge, Node: appconfig.NodeConfig{ID: "edge-002"}, RabbitMQ: appconfig.RabbitMQConfig{Mode: "managed", Install: true}}
	if err := admin.EnsureForNode(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(calls, "\n")
	for _, expected := range []string{
		"add_vhost /nodebridge-edge",
		"add_user nb-edge-002-local 1234",
		"change_password nb-edge-002-local 1234",
		"set_permissions -p /nodebridge-edge nb-edge-002-local .* .* .*",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in calls:\n%s", expected, joined)
		}
	}
}

func TestAdminMigratesExistingServerUserPassword(t *testing.T) {
	var calls []string
	admin := rabbitadmin.Admin{CLI: "rabbitmqctl.bat", Runner: func(_ context.Context, _ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		if args[0] == "list_vhosts" {
			return "/nodebridge-server\n", nil
		}
		if args[0] == "list_users" {
			return "nb-server-sync []\n", nil
		}
		return "", nil
	}}
	cfg := appconfig.Config{Mode: appconfig.ModeServer, Node: appconfig.NodeConfig{ID: "server-001"}, RabbitMQ: appconfig.RabbitMQConfig{Mode: "managed"}}
	if err := admin.EnsureForNode(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(calls, "\n")
	if strings.Contains(joined, "add_user") || !strings.Contains(joined, "change_password nb-server-sync 1234") {
		t.Fatalf("unexpected migration calls:\n%s", joined)
	}
}
