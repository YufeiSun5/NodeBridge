package rabbitmq_test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/rabbitmq/amqp091-go"
)

func TestIntegrationPublishConsume(t *testing.T) {
	url := os.Getenv("NODEBRIDGE_RABBITMQ_URL")
	if url == "" {
		t.Skip("NODEBRIDGE_RABBITMQ_URL not set")
	}

	conn, err := amqp091.Dial(url)
	if err != nil {
		t.Fatalf("dial RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("open channel: %v", err)
	}
	defer ch.Close()

	topology := rabbitmq.Topology{
		Exchanges: []rabbitmq.Exchange{{Name: "nodebridge.test.x", Kind: rabbitmq.ExchangeDirect, Durable: true}},
		Queues:    []rabbitmq.Queue{{Name: "nodebridge.test.q", Durable: true}},
		Bindings:  []rabbitmq.Binding{{Queue: "nodebridge.test.q", RoutingKey: "test", Exchange: "nodebridge.test.x"}},
	}
	if err := rabbitmq.InitializeTopology(ch, topology); err != nil {
		t.Fatalf("initialize topology: %v", err)
	}
	defer ch.QueueDelete("nodebridge.test.q", false, false, false)
	defer ch.ExchangeDelete("nodebridge.test.x", false, false)

	publisher, err := rabbitmq.NewPublisher(ch)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := publisher.Publish(ctx, rabbitmq.PublishRequest{
		Exchange:   "nodebridge.test.x",
		RoutingKey: "test",
		Body:       []byte(`{"event_id":"test"}`),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	status, err := rabbitmq.InspectQueue(ch, rabbitmq.Queue{Name: "nodebridge.test.q", Durable: true})
	if err != nil {
		t.Fatalf("inspect queue: %v", err)
	}
	if status.Messages < 1 {
		t.Fatalf("expected queued message, got %+v", status)
	}
}

func TestIntegrationPublishBatchFIFO(t *testing.T) {
	url := os.Getenv("NODEBRIDGE_RABBITMQ_URL")
	if url == "" {
		t.Skip("NODEBRIDGE_RABBITMQ_URL not set")
	}
	conn, err := amqp091.Dial(url)
	if err != nil {
		t.Fatalf("dial RabbitMQ: %v", err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("open channel: %v", err)
	}
	defer ch.Close()
	queue, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		t.Fatalf("declare isolated queue: %v", err)
	}
	publisher, err := rabbitmq.NewPublisher(ch)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	reqs := make([]rabbitmq.PublishRequest, 1000)
	for i := range reqs {
		reqs[i] = rabbitmq.PublishRequest{RoutingKey: queue.Name, Body: []byte(strconv.Itoa(i))}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := publisher.PublishBatch(ctx, reqs); err != nil {
		t.Fatalf("publish 1000 messages: %v", err)
	}
	for i := range reqs {
		delivery, ok, err := ch.Get(queue.Name, true)
		if err != nil || !ok {
			t.Fatalf("get message %d: ok=%t err=%v", i, ok, err)
		}
		if string(delivery.Body) != strconv.Itoa(i) {
			t.Fatalf("FIFO mismatch at %d: %q", i, delivery.Body)
		}
	}
	if _, ok, err := ch.Get(queue.Name, true); err != nil || ok {
		t.Fatalf("expected empty queue: message=%t err=%v", ok, err)
	}
}

func TestIntegrationPeekDistinctAndPreserve(t *testing.T) {
	url := os.Getenv("NODEBRIDGE_RABBITMQ_URL")
	if url == "" {
		t.Skip("NODEBRIDGE_RABBITMQ_URL not set")
	}
	conn, err := amqp091.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	queue, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := rabbitmq.NewPublisher(ch)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < 3; i++ {
		if err := publisher.Publish(ctx, rabbitmq.PublishRequest{RoutingKey: queue.Name, Body: []byte(strconv.Itoa(i))}); err != nil {
			t.Fatal(err)
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		messages, err := rabbitmq.PeekMessages(ctx, ch, queue.Name, 5)
		if err != nil || len(messages) != 3 {
			t.Fatalf("attempt=%d messages=%v err=%v", attempt, messages, err)
		}
		seen := map[string]bool{}
		for _, message := range messages {
			seen[string(message.Body)] = true
		}
		if len(seen) != 3 || !seen["0"] || !seen["1"] || !seen["2"] {
			t.Fatalf("duplicate or missing peek: %v", seen)
		}
	}
	seen := map[string]bool{}
	for len(seen) < 3 {
		delivery, ok, err := ch.Get(queue.Name, true)
		if err != nil || !ok || seen[string(delivery.Body)] {
			t.Fatalf("peek lost or duplicated messages: seen=%v ok=%t err=%v", seen, ok, err)
		}
		seen[string(delivery.Body)] = true
	}
	if _, ok, err := ch.Get(queue.Name, true); err != nil || ok {
		t.Fatalf("unexpected remaining message: ok=%t err=%v", ok, err)
	}
}
