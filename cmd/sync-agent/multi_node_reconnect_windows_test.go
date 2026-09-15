package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

func runOwnedRabbitReconnect(t *testing.T, ctx context.Context, root string, dbs []*sql.DB, tables, keys, values []string, execute func(int, string, ...any), all func(int, string) bool, absent func(int, int) bool) {
	t.Helper()
	var resources struct {
		Containers []string `json:"containers"`
		Endpoints  []struct {
			URL string `json:"local_url"`
		} `json:"endpoints"`
	}
	data, err := os.ReadFile(filepath.Join(root, "resources.json"))
	if err != nil || json.Unmarshal(data, &resources) != nil {
		t.Fatal("owned resource manifest required", err)
	}
	docker := "C:/Program Files/Docker/Docker/resources/bin/docker.exe"
	run := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, docker, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd.CombinedOutput()
	}
	broker := ""
	for _, id := range resources.Containers {
		out, err := run("inspect", "--format", "{{.Config.Image}}|{{.Name}}", id)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(out), "rabbitmq:") && strings.Contains(string(out), filepath.Base(root)) {
			broker = id
		}
	}
	if len(broker) != 64 {
		t.Fatal("owned RabbitMQ container not found")
	}
	wait := func(label string, predicate func() bool) {
		t.Helper()
		deadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(deadline) {
			if predicate() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("reconnect did not converge within 60 seconds: " + label)
	}
	var results []map[string]any
	for round, mode := range []string{"broker_restart", "transport_pause"} {
		base := 5000 + round*10
		for i := range dbs {
			execute(i, "INSERT INTO "+tables[i]+" ("+keys[i]+","+values[i]+",note) VALUES (?,1,'reconnect-base')", base+i)
			wait("baseline", func() bool { return all(base+i, "reconnect-base") })
		}
		var down, up []string
		if mode == "broker_restart" {
			down = []string{"exec", broker, "rabbitmqctl", "stop_app"}
			up = []string{"exec", broker, "rabbitmqctl", "start_app"}
		} else {
			down = []string{"pause", broker}
			up = []string{"unpause", broker}
		}
		if out, err := run(down...); err != nil {
			t.Fatal("fault injection", err, string(out))
		}
		func() {
			defer func() {
				if out, err := run(up...); err != nil {
					t.Errorf("restore broker: %v %s", err, out)
				}
			}()
			for i := range dbs {
				execute(i, "UPDATE "+tables[i]+" SET note=? WHERE "+keys[i]+"=?", mode, base+i)
				execute(i, "INSERT INTO "+tables[i]+" ("+keys[i]+","+values[i]+",note) VALUES (?,2,?)", base+3+i, mode)
			}
			// Exceed heartbeat detection and publish-confirm timeout without restarting Agents.
			time.Sleep(20 * time.Second)
		}()
		restored := time.Now()
		for i := range dbs {
			wait("offline update", func() bool { return all(base+i, mode) })
			wait("offline insert", func() bool { return all(base+3+i, mode) })
		}
		recovery := time.Since(restored)
		for i := 0; i < 6; i++ {
			writer := (i + 1) % 3
			execute(writer, "DELETE FROM "+tables[writer]+" WHERE "+keys[writer]+"=?", base+i)
			wait("post-recovery delete", func() bool { return absent(0, base+i) && absent(1, base+i) && absent(2, base+i) })
		}
		for i, endpoint := range resources.Endpoints {
			wait("queues drain", func() bool {
				conn, err := amqp091.Dial(endpoint.URL)
				if err != nil {
					return false
				}
				defer conn.Close()
				ch, err := conn.Channel()
				if err != nil {
					return false
				}
				defer ch.Close()
				queues := []string{"edge.upload.cdc.q"}
				if i == 1 {
					queues = []string{"server.cdc.ingress.q", "owned-multi-edge1.downlink.q", "owned-multi-edge2.downlink.q"}
				}
				for _, queue := range queues {
					q, err := ch.QueueDeclarePassive(queue, true, false, false, false, nil)
					if err != nil || q.Messages != 0 {
						return false
					}
				}
				return true
			})
		}
		results = append(results, map[string]any{"fault": mode, "outage_seconds": 20, "recovery_ms": recovery.Milliseconds(), "all_origin_crud": true, "queues_drained": true, "agent_restart": false})
		t.Logf("PASS: %s recovered all-origin inserts/updates in %s without Agent restart; cross-origin deletes and queue drain passed", mode, recovery.Round(time.Millisecond))
	}
	encoded, _ := json.MarshalIndent(map[string]any{"passed": true, "results": results, "scope": fmt.Sprintf("owned broker %s; three Windows Agents", broker)}, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "reconnect-evidence.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
}
