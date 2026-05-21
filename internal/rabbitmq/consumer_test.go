package rabbitmq_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
)

func TestConsumerAckOnSuccess(t *testing.T) {
	msg := &fakeIncomingMessage{body: []byte("ok")}
	consumer := rabbitmq.Consumer{}

	err := consumer.Handle(context.Background(), msg, func(ctx context.Context, body []byte) error {
		if string(body) != "ok" {
			t.Fatalf("unexpected body %q", body)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !msg.acked || msg.nacked {
		t.Fatalf("expected ack only, got %+v", msg)
	}
}

func TestConsumerNackOnHandlerError(t *testing.T) {
	msg := &fakeIncomingMessage{}
	consumer := rabbitmq.Consumer{RequeueOnError: true}

	err := consumer.Handle(context.Background(), msg, func(ctx context.Context, body []byte) error {
		return errors.New("handler failed")
	})
	if err == nil {
		t.Fatal("expected handler error")
	}
	if !msg.nacked || !msg.requeue {
		t.Fatalf("expected nack with requeue, got %+v", msg)
	}
}

func TestConsumerNackOnPanic(t *testing.T) {
	msg := &fakeIncomingMessage{}
	consumer := rabbitmq.Consumer{}

	err := consumer.Handle(context.Background(), msg, func(ctx context.Context, body []byte) error {
		panic("boom")
	})
	if err == nil {
		t.Fatal("expected panic error")
	}
	if !msg.nacked || msg.requeue {
		t.Fatalf("expected nack without requeue, got %+v", msg)
	}
}

type fakeIncomingMessage struct {
	body    []byte
	acked   bool
	nacked  bool
	requeue bool
}

func (m *fakeIncomingMessage) Body() []byte {
	return m.body
}

func (m *fakeIncomingMessage) Ack(multiple bool) error {
	m.acked = true
	return nil
}

func (m *fakeIncomingMessage) Nack(multiple, requeue bool) error {
	m.nacked = true
	m.requeue = requeue
	return nil
}
