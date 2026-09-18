package rabbitmq_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	rabbitadmin "github.com/YufeiSun5/NodeBridge/internal/installer/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/rabbitmq/amqp091-go"
)

// Requires a disposable broker created solely for this test, never a live vhost.
func TestOwnedBrokerEdgeAlignmentPermissions(t *testing.T) {
	container := os.Getenv("NODEBRIDGE_OWNED_PERMISSION_BROKER")
	nativeCLI := os.Getenv("NODEBRIDGE_OWNED_WINDOWS_RABBITMQ_CLI")
	if container == "" && nativeCLI == "" {
		t.Skip("requires an owned nb-permission-test-* RabbitMQ container")
	}
	if nativeCLI == "" && !strings.HasPrefix(container, "nb-permission-test-") {
		t.Fatal("owned broker name required")
	}
	if nativeCLI != "" && (runtime.GOOS != "windows" || os.Getenv("RABBITMQ_NODENAME") != "nb_permission_patch@localhost" || os.Getenv("ERL_EPMD_PORT") != "14369") {
		t.Fatal("owned native broker identity required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin := rabbitadmin.Admin{CLI: "rabbitmqctl", Runner: func(ctx context.Context, _ string, args ...string) (string, error) {
		output, err := exec.CommandContext(ctx, "docker", append([]string{"exec", container, "rabbitmqctl"}, args...)...).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("owned rabbitmqctl %s failed: %w", args[0], err)
		}
		return string(output), nil
	}}
	if nativeCLI != "" {
		admin = rabbitadmin.NewAdmin()
		admin.CLI = nativeCLI
	}
	if err := admin.EnsureServerEdgeUser(ctx, "edge-001"); err != nil {
		t.Fatal(err)
	}
	endpoint, err := url.Parse(os.Getenv("NODEBRIDGE_OWNED_PERMISSION_URL"))
	if err != nil || endpoint.Scheme != "amqp" || endpoint.Hostname() != "127.0.0.1" || endpoint.Port() == "" {
		t.Fatal("owned loopback AMQP endpoint required")
	}
	endpoint.User = url.UserPassword("nb-edge-001", appconfig.ManagedRabbitMQPassword)
	endpoint.Path = "//nodebridge-server"
	endpoint.RawPath = "/%2Fnodebridge-server"
	connect := func() *amqp091.Connection {
		t.Helper()
		conn, err := amqp091.DialConfig(endpoint.String(), amqp091.Config{Dial: amqp091.DefaultDial(3 * time.Second)})
		if err != nil {
			t.Fatal("restricted edge connection failed", err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}
	left, right := connect(), connect()
	// Both roles deliberately use the restricted identity to exercise every
	// control operation without accidentally borrowing administrator privileges.
	rule := rules.SyncRule{ID: "permissions"}
	done := make(chan error, 1)
	go func() {
		control, err := alignment.OpenPairControl(ctx, right, "server-001", "edge-001", rule, false)
		if err == nil {
			defer control.Close()
			var peer string
			err = control.Exchange(ctx, "permission_probe", "server", &peer)
			if err == nil && peer != "edge" {
				err = fmt.Errorf("wrong edge payload: %q", peer)
			}
		}
		done <- err
	}()
	control, err := alignment.OpenPairControl(ctx, left, "edge-001", "server-001", rule, true)
	if err != nil {
		cancel()
		<-done
		t.Fatal(err)
	}
	defer control.Close()
	var peer string
	if err := control.Exchange(ctx, "permission_probe", "edge", &peer); err != nil || peer != "server" {
		t.Fatal("control exchange failed", err, peer)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	ch, err := left.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	for _, suffix := range []string{"frames", "replies"} {
		queue := "nb.alignment." + strings.Repeat("a", 64) + "." + suffix
		if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
			t.Fatal(err)
		}
		if err := ch.PublishWithContext(ctx, "", queue, false, false, amqp091.Publishing{DeliveryMode: amqp091.Persistent, Body: []byte(suffix)}); err != nil {
			t.Fatal(err)
		}
		var received bool
		for !received && ctx.Err() == nil {
			delivery, ok, err := ch.Get(queue, false)
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				if string(delivery.Body) != suffix {
					t.Fatal("snapshot payload changed")
				}
				if err := delivery.Ack(false); err != nil {
					t.Fatal(err)
				}
				received = true
			}
		}
		if !received {
			t.Fatal("snapshot message not received")
		}
	}
	denied, err := left.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Close()
	if _, err := denied.QueueDeclare("unrelated.business.queue", true, false, false, false, nil); err == nil {
		t.Fatal("restricted identity could configure an unrelated queue")
	}
}
