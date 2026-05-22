package rabbitmq_test

import (
	"context"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/rabbitmq/amqp091-go"
)

func TestPeekMessagesRequeues(t *testing.T) {
	getter := &fakeQueueGetter{
		deliveries: []amqp091.Delivery{
			{Body: []byte("one"), ContentType: "text/plain", Acknowledger: &recordingAcknowledger{}},
			{Body: []byte("two"), ContentType: "text/plain", Acknowledger: &recordingAcknowledger{}},
		},
	}

	messages, err := rabbitmq.PeekMessages(context.Background(), getter, "server.dead.q", 2)
	if err != nil {
		t.Fatalf("PeekMessages returned error: %v", err)
	}
	if len(messages) != 2 || string(messages[0].Body) != "one" || string(messages[1].Body) != "two" {
		t.Fatalf("unexpected messages %+v", messages)
	}
	for i, delivery := range getter.deliveries {
		ack := delivery.Acknowledger.(*recordingAcknowledger)
		if !ack.nacked || !ack.requeue {
			t.Fatalf("delivery %d must be requeued, got %+v", i, ack)
		}
	}
}

type fakeQueueGetter struct {
	deliveries []amqp091.Delivery
	index      int
}

func (g *fakeQueueGetter) Get(queue string, autoAck bool) (amqp091.Delivery, bool, error) {
	if g.index >= len(g.deliveries) {
		return amqp091.Delivery{}, false, nil
	}
	delivery := g.deliveries[g.index]
	g.index++
	return delivery, true, nil
}

type recordingAcknowledger struct {
	nacked  bool
	requeue bool
}

func (a *recordingAcknowledger) Ack(tag uint64, multiple bool) error {
	return nil
}

func (a *recordingAcknowledger) Nack(tag uint64, multiple bool, requeue bool) error {
	a.nacked = true
	a.requeue = requeue
	return nil
}

func (a *recordingAcknowledger) Reject(tag uint64, requeue bool) error {
	return nil
}
