package rabbitmq_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	rabbitadmin "github.com/YufeiSun5/NodeBridge/internal/installer/rabbitmq"
)

func TestServerEdgePermissionsCoverAlignmentAndRemainBounded(t *testing.T) {
	var permissions []string
	admin := rabbitadmin.Admin{CLI: "rabbitmqctl.bat", Runner: func(_ context.Context, _ string, args ...string) (string, error) {
		if args[0] == "set_permissions" {
			permissions = append([]string(nil), args[4:]...)
		}
		return "", nil
	}}
	if err := admin.EnsureServerEdgeUser(context.Background(), "edge.001"); err != nil {
		t.Fatal(err)
	}
	if len(permissions) != 3 {
		t.Fatalf("permissions = %v", permissions)
	}
	configure, write, read := regexp.MustCompile(permissions[0]), regexp.MustCompile(permissions[1]), regexp.MustCompile(permissions[2])
	for _, queue := range []string{
		"nb.alignment.session." + strings.Repeat("a", 32),
		"nb.alignment.offer." + strings.Repeat("b", 64),
		"nb.alignment." + strings.Repeat("c", 64) + ".frames",
		"nb.alignment." + strings.Repeat("d", 64) + ".replies",
	} {
		if !configure.MatchString(queue) || !read.MatchString(queue) {
			t.Errorf("alignment queue denied: %s", queue)
		}
	}
	for _, exchange := range []string{"amq.default", "server.ingress.exchange"} {
		if !write.MatchString(exchange) {
			t.Errorf("required exchange denied: %s", exchange)
		}
	}
	if !read.MatchString("edge.001.downlink.queue") {
		t.Fatal("own downlink denied")
	}
	for _, queue := range []string{"edge-002.downlink.queue", "edgeX001.downlink.queue", "prefix.edge.001.downlink.queue", "business", "nb.alignment.session.invalid", "nb.alignment." + strings.Repeat("a", 64) + ".frames.other"} {
		if configure.MatchString(queue) || read.MatchString(queue) {
			t.Errorf("unrelated queue allowed: %s", queue)
		}
	}
	for _, exchange := range []string{"amq.topic", "xamq.default", "amq.default.other", "xserver.ingress.exchange", "business"} {
		if write.MatchString(exchange) {
			t.Errorf("unrelated exchange allowed: %s", exchange)
		}
	}
}

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
