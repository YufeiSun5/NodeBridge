package rabbitmq

import (
	"context"
	"errors"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

type PublishChannel interface {
	Confirm(noWait bool) error
	NotifyPublish(confirm chan amqp091.Confirmation) chan amqp091.Confirmation
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error
}

type Publisher struct {
	channel PublishChannel
}

func NewPublisher(channel PublishChannel) (*Publisher, error) {
	if err := channel.Confirm(false); err != nil {
		return nil, fmt.Errorf("enable publisher confirm: %w", err)
	}
	return &Publisher{channel: channel}, nil
}

type PublishRequest struct {
	Exchange    string
	RoutingKey  string
	Body        []byte
	ContentType string
	Headers     amqp091.Table
}

func (p *Publisher) Publish(ctx context.Context, req PublishRequest) error {
	confirmations := p.channel.NotifyPublish(make(chan amqp091.Confirmation, 1))
	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	if err := p.channel.PublishWithContext(ctx, req.Exchange, req.RoutingKey, true, false, amqp091.Publishing{
		DeliveryMode: amqp091.Persistent,
		ContentType:  contentType,
		Headers:      req.Headers,
		Body:         req.Body,
	}); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	select {
	case confirmation := <-confirmations:
		if !confirmation.Ack {
			return errors.New("publish not acknowledged by broker")
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait publish confirm: %w", ctx.Err())
	}
}
