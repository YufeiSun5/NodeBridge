package rabbitmq

import (
	"context"
	"fmt"
)

type IncomingMessage interface {
	Body() []byte
	Ack(multiple bool) error
	Nack(multiple, requeue bool) error
}

type MessageHandler func(ctx context.Context, body []byte) error

type Consumer struct {
	RequeueOnError bool
}

func (c Consumer) Handle(ctx context.Context, msg IncomingMessage, handler MessageHandler) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("consumer handler panic: %v", recovered)
			if nackErr := msg.Nack(false, c.RequeueOnError); nackErr != nil {
				err = fmt.Errorf("%w; nack failed: %v", err, nackErr)
			}
		}
	}()

	if err := handler(ctx, msg.Body()); err != nil {
		if nackErr := msg.Nack(false, c.RequeueOnError); nackErr != nil {
			return fmt.Errorf("handler failed: %w; nack failed: %v", err, nackErr)
		}
		return err
	}

	if err := msg.Ack(false); err != nil {
		return fmt.Errorf("ack message: %w", err)
	}
	return nil
}

func (c Consumer) HandleBatch(ctx context.Context, messages []IncomingMessage, handler MessageHandler) error {
	for index, msg := range messages {
		if err := c.handleBatchOne(ctx, msg, handler); err != nil {
			if nackErr := nackRest(messages[index+1:], c.RequeueOnError); nackErr != nil {
				return fmt.Errorf("%w; nack rest failed: %v", err, nackErr)
			}
			return err
		}
	}
	return nil
}

func (c Consumer) handleBatchOne(ctx context.Context, msg IncomingMessage, handler MessageHandler) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("consumer handler panic: %v", recovered)
			if nackErr := msg.Nack(false, c.RequeueOnError); nackErr != nil {
				err = fmt.Errorf("%w; nack failed: %v", err, nackErr)
			}
		}
	}()

	if err := handler(ctx, msg.Body()); err != nil {
		if nackErr := msg.Nack(false, c.RequeueOnError); nackErr != nil {
			return fmt.Errorf("handler failed: %w; nack failed: %v", err, nackErr)
		}
		return err
	}
	if err := msg.Ack(false); err != nil {
		return fmt.Errorf("ack message: %w", err)
	}
	return nil
}

func nackRest(messages []IncomingMessage, requeue bool) error {
	for _, msg := range messages {
		if err := msg.Nack(false, requeue); err != nil {
			return err
		}
	}
	return nil
}
