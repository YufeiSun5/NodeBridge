package rabbitmq_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/rabbitmq/amqp091-go"
)

func TestPublisherPublishAck(t *testing.T) {
	channel := newFakePublishChannel()
	publisher, err := rabbitmq.NewPublisher(channel)
	if err != nil {
		t.Fatalf("NewPublisher returned error: %v", err)
	}

	err = publisher.Publish(context.Background(), rabbitmq.PublishRequest{
		Exchange:   "events.x",
		RoutingKey: "events",
		Body:       []byte(`{"ok":true}`),
	})
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	if !channel.confirmEnabled {
		t.Fatal("expected confirm mode")
	}
	if channel.published.DeliveryMode != amqp091.Persistent {
		t.Fatalf("expected persistent message, got %d", channel.published.DeliveryMode)
	}
}

func TestPublisherPublishNack(t *testing.T) {
	channel := newFakePublishChannel()
	channel.ack = false
	publisher, err := rabbitmq.NewPublisher(channel)
	if err != nil {
		t.Fatalf("NewPublisher returned error: %v", err)
	}

	err = publisher.Publish(context.Background(), rabbitmq.PublishRequest{Exchange: "events.x", RoutingKey: "events"})
	if err == nil {
		t.Fatal("expected nack error")
	}
}

func TestPublisherPublishError(t *testing.T) {
	channel := newFakePublishChannel()
	channel.publishErr = errors.New("publish failed")
	publisher, err := rabbitmq.NewPublisher(channel)
	if err != nil {
		t.Fatalf("NewPublisher returned error: %v", err)
	}

	err = publisher.Publish(context.Background(), rabbitmq.PublishRequest{Exchange: "events.x", RoutingKey: "events"})
	if err == nil {
		t.Fatal("expected publish error")
	}
}

type fakePublishChannel struct {
	confirmEnabled bool
	ack            bool
	publishErr     error
	published      amqp091.Publishing
	confirmations  chan amqp091.Confirmation
}

func newFakePublishChannel() *fakePublishChannel {
	return &fakePublishChannel{ack: true, confirmations: make(chan amqp091.Confirmation, 1)}
}

func (c *fakePublishChannel) Confirm(noWait bool) error {
	c.confirmEnabled = true
	return nil
}

func (c *fakePublishChannel) NotifyPublish(confirm chan amqp091.Confirmation) chan amqp091.Confirmation {
	c.confirmations = confirm
	return confirm
}

func (c *fakePublishChannel) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error {
	if c.publishErr != nil {
		return c.publishErr
	}
	c.published = msg
	c.confirmations <- amqp091.Confirmation{Ack: c.ack}
	return nil
}
