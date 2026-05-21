package rabbitmq_test

import (
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/rabbitmq/amqp091-go"
)

func TestInspectQueue(t *testing.T) {
	inspector := fakeQueueInspector{queue: amqp091.Queue{Name: "events.q", Messages: 7, Consumers: 2}}

	status, err := rabbitmq.InspectQueue(inspector, rabbitmq.Queue{Name: "events.q", Durable: true})
	if err != nil {
		t.Fatalf("InspectQueue returned error: %v", err)
	}
	if status.Name != "events.q" || status.Messages != 7 || status.Consumers != 2 {
		t.Fatalf("unexpected status %+v", status)
	}
}

type fakeQueueInspector struct {
	queue amqp091.Queue
}

func (i fakeQueueInspector) QueueDeclarePassive(name string, durable, autoDelete, exclusive, noWait bool, args amqp091.Table) (amqp091.Queue, error) {
	return i.queue, nil
}
