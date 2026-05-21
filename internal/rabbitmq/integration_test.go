package rabbitmq_test

import (
	"context"
	"os"
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
