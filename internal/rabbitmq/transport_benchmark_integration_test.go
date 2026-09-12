package rabbitmq_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestInspectLabQueueHeadReal(t *testing.T) {
	url, queue := os.Getenv("NODEBRIDGE_AMQP_TRANSPORT_BENCH_URL"), os.Getenv("NODEBRIDGE_AMQP_INSPECT_QUEUE")
	if url == "" || queue == "" {
		t.Skip("set lab URL and NODEBRIDGE_AMQP_INSPECT_QUEUE for requeued metadata inspection")
	}
	if queue != "edge-001.downlink.q" && queue != "server.cdc.ingress.q" {
		t.Fatal("only the two lab transport queues may be inspected")
	}
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	msg, ok, err := ch.Get(queue, false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Log("queue empty")
		return
	}
	defer func() {
		if err := msg.Nack(false, true); err != nil {
			t.Error(err)
		}
	}()
	var metadata struct {
		EventID   string `json:"event_id"`
		Table     string `json:"table_name"`
		EventType string `json:"event_type"`
	}
	if err := json.Unmarshal(msg.Body, &metadata); err != nil {
		t.Fatal("invalid event JSON")
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("requeued queue=%s bytes=%d redelivered=%v metadata=%s", queue, len(msg.Body), msg.Redelivered, data)
}

func TestTransportPullPushReal(t *testing.T) {
	url := os.Getenv("NODEBRIDGE_AMQP_TRANSPORT_BENCH_URL")
	if url == "" {
		t.Skip("set NODEBRIDGE_AMQP_TRANSPORT_BENCH_URL for isolated transport measurements")
	}
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	reports := []map[string]any{}
	for _, mode := range []string{"get", "consume"} {
		ch, err := conn.Channel()
		if err != nil {
			t.Fatal(err)
		}
		defer ch.Close()
		name := fmt.Sprintf("nodebridge.transport-bench.%d.%s", time.Now().UnixNano(), mode)
		// These isolated queues expire after two unused minutes, including on test failure.
		q, err := ch.QueueDeclare(name, true, false, false, false, amqp.Table{"x-expires": int32(120000)})
		if err != nil {
			t.Fatal(err)
		}
		if err := ch.Confirm(false); err != nil {
			t.Fatal(err)
		}
		const count = 500
		confirmed := ch.NotifyPublish(make(chan amqp.Confirmation, count))
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		for i := 0; i < count; i++ {
			body, err := json.Marshal(map[string]any{"sequence": i, "padding": strings.Repeat("x", 1024)})
			if err != nil {
				t.Fatal(err)
			}
			if err := ch.PublishWithContext(ctx, "", q.Name, true, false, amqp.Publishing{DeliveryMode: amqp.Persistent, Body: body}); err != nil {
				t.Fatal(err)
			}
		}
		for i := 0; i < count; i++ {
			select {
			case c, ok := <-confirmed:
				if !ok || !c.Ack {
					t.Fatal("publisher confirm failed")
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
		if err := ch.Qos(64, 0, false); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		var deliveries <-chan amqp.Delivery
		if mode == "consume" {
			deliveries, err = ch.Consume(q.Name, name, false, false, false, false, nil)
			if err != nil {
				t.Fatal(err)
			}
		}
		for i := 0; i < count; i++ {
			var msg amqp.Delivery
			var ok bool
			if mode == "get" {
				msg, ok, err = ch.Get(q.Name, false)
			} else {
				select {
				case msg, ok = <-deliveries:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			if err != nil || !ok {
				t.Fatalf("message %d missing: %v", i, err)
			}
			var body struct {
				Sequence int    `json:"sequence"`
				Padding  string `json:"padding"`
			}
			if err := json.Unmarshal(msg.Body, &body); err != nil || body.Sequence != i || len(body.Padding) != 1024 {
				t.Fatalf("message %d changed or out of order", i)
			}
			if err := msg.Ack(false); err != nil {
				t.Fatal(err)
			}
		}
		if mode == "consume" {
			if err := ch.Cancel(name, false); err != nil {
				t.Fatal(err)
			}
		}
		state, err := ch.QueueDeclarePassive(q.Name, true, false, false, false, nil)
		if err != nil || state.Messages != 0 {
			t.Fatalf("queue not drained: %+v %v", state, err)
		}
		elapsed := time.Since(start)
		reports = append(reports, map[string]any{"mode": mode, "messages": count, "elapsed_ms": float64(elapsed.Microseconds()) / 1000, "messages_per_second": float64(count) / elapsed.Seconds(), "manual_ack": true, "order_verified": true, "queue": q.Name})
		t.Logf("%s: %d messages in %v, %.2f messages/s", mode, count, elapsed, float64(count)/elapsed.Seconds())
	}
	if path := os.Getenv("NODEBRIDGE_AMQP_TRANSPORT_BENCH_REPORT"); path != "" {
		b, err := json.MarshalIndent(map[string]any{"at": time.Now(), "passed": true, "measurements": reports}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
