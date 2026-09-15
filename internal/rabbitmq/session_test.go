package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

func TestSessionClosedAndCancelledNeverDial(t *testing.T) {
	s := &Session{url: "invalid"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Publish(ctx, PublishRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	_ = s.Close()
	if err := s.Publish(context.Background(), PublishRequest{}); !errors.Is(err, amqp091.ErrClosed) {
		t.Fatalf("closed session reopened: %v", err)
	}
}

type hiddenConfirms struct{ *amqp091.Channel }

func (c hiddenConfirms) NotifyPublish(ch chan amqp091.Confirmation) chan amqp091.Confirmation {
	return ch
}

func TestIntegrationSessionRecreatesTransportAndUncertainConfirm(t *testing.T) {
	url := os.Getenv("NODEBRIDGE_RABBITMQ_URL")
	if url == "" {
		t.Skip("owned RabbitMQ fixture required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := DialSession(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	queue := fmt.Sprintf("nodebridge.reconnect.%d", time.Now().UnixNano())
	err = s.WithChannel(ctx, func(ch *amqp091.Channel) error {
		_, err := ch.QueueDeclare(queue, true, false, false, false, nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = s.WithChannel(ctx, func(ch *amqp091.Channel) error { _, err := ch.QueueDelete(queue, false, false, false); return err })
	}()
	req := PublishRequest{RoutingKey: queue, Body: []byte("same-event-id")}
	old := s.conn
	_ = old.Channel.Close()
	if err := s.Publish(ctx, req); err != nil {
		t.Fatal("closed channel not replaced", err)
	}
	if s.conn == old {
		t.Fatal("old transport reused")
	}
	var delivery amqp091.Delivery
	err = s.WithChannel(ctx, func(ch *amqp091.Channel) error {
		var ok bool
		var err error
		delivery, ok, err = ch.Get(queue, false)
		if err == nil && !ok {
			return errors.New("missing delivery")
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.conn.Conn.CloseDeadline(time.Now())
	if err := s.WithChannel(ctx, func(ch *amqp091.Channel) error {
		redelivered, ok, err := ch.Get(queue, false)
		if err != nil {
			return err
		}
		if !ok || !redelivered.Redelivered {
			return errors.New("unacked delivery was lost")
		}
		if delivery.Ack(false) == nil {
			return errors.New("old delivery ack used replacement channel")
		}
		return redelivered.Ack(false)
	}); err != nil {
		t.Fatal(err)
	}

	// Broker receives the message, but its confirmation is hidden from the caller.
	s.publisher, err = NewPublisher(hiddenConfirms{s.conn.Channel})
	if err != nil {
		t.Fatal(err)
	}
	uncertain, stop := context.WithTimeout(ctx, 250*time.Millisecond)
	err = s.Publish(uncertain, req)
	stop()
	if err == nil || s.conn != nil || s.publisher != nil {
		t.Fatal("uncertain publish must discard transport", err)
	}
	if err := s.Publish(ctx, req); err != nil {
		t.Fatal("retry on fresh publisher failed", err)
	}
	for i := 0; i < 2; i++ {
		if err := s.WithChannel(ctx, func(ch *amqp091.Channel) error {
			msg, ok, err := ch.Get(queue, false)
			if err != nil {
				return err
			}
			if !ok || string(msg.Body) != "same-event-id" {
				return errors.New("uncertain send was lost or changed identity")
			}
			return msg.Ack(false)
		}); err != nil {
			t.Fatal(err)
		}
	}
}
