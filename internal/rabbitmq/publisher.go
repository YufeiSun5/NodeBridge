package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rabbitmq/amqp091-go"
)

type PublishChannel interface {
	Confirm(noWait bool) error
	NotifyPublish(confirm chan amqp091.Confirmation) chan amqp091.Confirmation
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error
}

const publisherConfirmWindow = 64

type Publisher struct {
	channel       PublishChannel
	confirmations <-chan amqp091.Confirmation
	mu            sync.Mutex
	pending       int
	terminalErr   error
}

// NewPublisher requires a channel not shared with other publishers.
func NewPublisher(channel PublishChannel) (*Publisher, error) {
	if err := channel.Confirm(false); err != nil {
		return nil, fmt.Errorf("enable publisher confirm: %w", err)
	}
	confirmations := channel.NotifyPublish(make(chan amqp091.Confirmation, publisherConfirmWindow))
	return &Publisher{channel: channel, confirmations: confirmations}, nil
}

type PublishRequest struct {
	Exchange    string
	RoutingKey  string
	Body        []byte
	ContentType string
	Headers     amqp091.Table
}

func (p *Publisher) Publish(ctx context.Context, req PublishRequest) error {
	return p.PublishBatch(ctx, []PublishRequest{req})
}

func (p *Publisher) PublishBatch(ctx context.Context, reqs []PublishRequest) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(reqs) == 0 {
		return nil
	}
	if p.terminalErr != nil {
		return p.terminalErr
	}
	// A failed call owns these confirms; never count them toward the next call.
	if _, err := p.drainConfirms(ctx); err != nil {
		return err
	}
	for start := 0; start < len(reqs); start += publisherConfirmWindow {
		end := min(start+publisherConfirmWindow, len(reqs))
		for _, req := range reqs[start:end] {
			if err := p.publish(ctx, req); err != nil {
				return fmt.Errorf("publish message: %w", err)
			}
		}
		nacked, err := p.drainConfirms(ctx)
		if nacked {
			return errors.Join(errors.New("publish not acknowledged by broker"), err)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Publisher) publish(ctx context.Context, req PublishRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := req.Headers.Validate(); err != nil {
		return err
	}
	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/json"
	}
	err := p.channel.PublishWithContext(ctx, req.Exchange, req.RoutingKey, true, false, amqp091.Publishing{
		DeliveryMode: amqp091.Persistent,
		ContentType:  contentType,
		Headers:      req.Headers,
		Body:         req.Body,
	})
	if err != nil {
		// amqp091-go v1.11 checks context before I/O. Other send errors can
		// follow partial writes and roll back its confirm tag via unpublish.
		if err != ctx.Err() {
			p.terminalErr = fmt.Errorf("publisher unusable after send failure; recreate channel and publisher: %w", err)
		}
		return err
	}
	p.pending++
	return nil
}

func (p *Publisher) drainConfirms(ctx context.Context) (nacked bool, err error) {
	for p.pending > 0 {
		if err := ctx.Err(); err != nil {
			return nacked, fmt.Errorf("wait publish confirm: %w", err)
		}
		select {
		case confirmation, ok := <-p.confirmations:
			if !ok {
				p.terminalErr = fmt.Errorf("publisher confirmation channel closed: %w", amqp091.ErrClosed)
				return nacked, p.terminalErr
			}
			p.pending--
			nacked = nacked || !confirmation.Ack
		case <-ctx.Done():
			return nacked, fmt.Errorf("wait publish confirm: %w", ctx.Err())
		}
	}
	return nacked, nil
}
