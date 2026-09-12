package datasyncui

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
	"github.com/YufeiSun5/NodeBridge/internal/queueaudit"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/rabbitmq/amqp091-go"
)

func TestQueueAuditMCPRealBroker(t *testing.T) {
	raw := os.Getenv("NODEBRIDGE_REMEDIATION_RABBITMQ_URL")
	if raw == "" {
		t.Skip("isolated broker URL required")
	}
	u, err := url.Parse(raw)
	if err != nil || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() {
		t.Fatal("loopback broker required")
	}
	conn, err := amqp091.Dial(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	// Only use this test on its explicitly dedicated disposable broker.
	queue := "server.cdc.ingress.q"
	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		t.Fatal(err)
	}
	defer ch.QueueDelete(queue, false, false, false)
	defer ch.QueueDelete(queue+".quarantine", false, false, false)
	dir := t.TempDir()
	configPath, rulesPath := filepath.Join(dir, "config.yaml"), filepath.Join(dir, "rules.yaml")
	service, err := NewMCPService(configPath, rulesPath, "", true)
	if err != nil {
		t.Fatal(err)
	}
	patch, _ := json.Marshal(map[string]any{"patch": map[string]any{"mode": "server", "node": map[string]any{"id": "owned-queue-audit"}, "mysql": map[string]any{"database": "owned_unused"}, "rabbitmq": map[string]any{"server_url": raw}}})
	if out := callMCP(t, service, "nodebridge_save_config_patch", string(patch)); strings.Contains(out, `"isError":true`) {
		t.Fatal(out)
	}
	set := rules.RuleSet{Rules: []rules.SyncRule{{ID: "owned", DatabaseName: "owned_source", TableName: "items", PrimaryKeys: []string{"id"}, Direction: rules.DirectionEdgeToServer, ConflictPolicy: rules.ConflictNone, DeleteMode: rules.DeleteHard, Enable: false}}}
	if err := rules.SaveFile(rulesPath, set); err != nil {
		t.Fatal(err)
	}
	publisher, err := rabbitmq.NewPublisher(ch)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, id := range []string{"unrelated", "owned-event"} {
		body, _ := json.Marshal(map[string]any{"event_id": id, "event_type": "DELETE", "database_name": "owned_source", "table_name": "items", "origin_node_id": "owned-edge", "source_node_id": "owned-edge", "primary_key": map[string]any{"id": 1}})
		if err := publisher.Publish(ctx, rabbitmq.PublishRequest{RoutingKey: queue, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	args := `{"queue":"server.cdc.ingress.q","rule_id":"owned","event_id":"owned-event"}`
	if out := callMCP(t, service, "nodebridge_queue_event_plan", args); strings.Contains(out, `"isError":true`) {
		t.Fatal(out)
	}
	files, err := filepath.Glob(filepath.Join(dir, "queue-audit", "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("plans=%v %v", files, err)
	}
	planID := strings.TrimSuffix(filepath.Base(files[0]), ".json")
	journal := queueaudit.FileJournal{Directory: filepath.Join(dir, "queue-audit")}
	planned, err := journal.Load(planID)
	if err != nil || planned.State != "planned" {
		t.Fatalf("%+v %v", planned, err)
	}
	applyArgs := `{"plan_id":"` + planID + `","confirm":true}`
	if out := callMCP(t, service, "nodebridge_queue_event_apply", `{"plan_id":"`+planID+`"}`); !strings.Contains(out, `"isError":true`) {
		t.Fatal("missing confirmation accepted")
	}
	release, err := agentstate.Acquire(configPath, "")
	if err != nil {
		t.Fatal(err)
	}
	blocked := callMCP(t, service, "nodebridge_queue_event_apply", applyArgs)
	release()
	if !strings.Contains(blocked, `"isError":true`) {
		t.Fatal("running agent accepted")
	}
	for i := 0; i < 2; i++ {
		if out := callMCP(t, service, "nodebridge_queue_event_apply", applyArgs); strings.Contains(out, `"isError":true`) || !strings.Contains(out, "acked") {
			t.Fatal(out)
		}
	}
	if out := callMCP(t, service, "nodebridge_queue_event_audit", `{"plan_id":"`+planID+`"}`); strings.Contains(out, `"isError":true`) || !strings.Contains(out, "acked") {
		t.Fatal(out)
	}
	delivery, ok, err := ch.Get(queue+".quarantine", true)
	if err != nil || !ok || delivery.Headers["nodebridge_quarantine_plan_id"] != planID || !strings.Contains(string(delivery.Body), "owned-event") || delivery.DeliveryMode != amqp091.Persistent {
		t.Fatalf("invalid durable copy: ok=%t err=%v", ok, err)
	}
	if _, ok, err := ch.Get(queue+".quarantine", true); err != nil || ok {
		t.Fatalf("duplicate on confirmed retry: %t %v", ok, err)
	}
	delivery, ok, err = ch.Get(queue, true)
	if err != nil || !ok || !strings.Contains(string(delivery.Body), "unrelated") {
		t.Fatalf("unrelated message lost: %t %v", ok, err)
	}
	if _, ok, err := ch.Get(queue, true); err != nil || ok {
		t.Fatalf("original remains: %t %v", ok, err)
	}
}
