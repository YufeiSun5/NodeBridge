package rabbitmq_test

import (
	"context"
	"errors"
	"strings"
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

func TestConsumerHandleBatchAcksInOrder(t *testing.T) {
	messages := []*fakeIncomingMessage{{body: []byte("1")}, {body: []byte("2")}, {body: []byte("3")}}
	consumer := rabbitmq.Consumer{}
	var seen []string

	err := consumer.HandleBatch(context.Background(), incoming(messages), func(ctx context.Context, body []byte) error {
		seen = append(seen, string(body))
		return nil
	})
	if err != nil {
		t.Fatalf("HandleBatch returned error: %v", err)
	}
	if got := strings.Join(seen, ","); got != "1,2,3" {
		t.Fatalf("unexpected order %s", got)
	}
	for i, msg := range messages {
		if !msg.acked || msg.nacked {
			t.Fatalf("message %d expected ack only, got %+v", i, msg)
		}
	}
}

func TestConsumerHandleBatchNacksFailureAndRest(t *testing.T) {
	messages := []*fakeIncomingMessage{{body: []byte("1")}, {body: []byte("2")}, {body: []byte("3")}}
	consumer := rabbitmq.Consumer{RequeueOnError: true}

	err := consumer.HandleBatch(context.Background(), incoming(messages), func(ctx context.Context, body []byte) error {
		if string(body) == "2" {
			return errors.New("failed")
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected batch failure")
	}
	if !messages[0].acked || messages[0].nacked {
		t.Fatalf("first message should be acked, got %+v", messages[0])
	}
	for i := 1; i < len(messages); i++ {
		if messages[i].acked || !messages[i].nacked || !messages[i].requeue {
			t.Fatalf("message %d should be requeue nacked, got %+v", i, messages[i])
		}
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

func incoming(messages []*fakeIncomingMessage) []rabbitmq.IncomingMessage {
	result := make([]rabbitmq.IncomingMessage, 0, len(messages))
	for _, msg := range messages {
		result = append(result, msg)
	}
	return result
}
