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
type BatchCommitHandler func(ctx context.Context, bodies [][]byte) (int, error)

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

func (c Consumer) HandleBatchCommit(ctx context.Context, messages []IncomingMessage, handler BatchCommitHandler) (err error) {
	bodies := make([][]byte, 0, len(messages))
	for _, msg := range messages {
		bodies = append(bodies, msg.Body())
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("consumer batch handler panic: %v", recovered)
			if nackErr := nackRest(messages, c.RequeueOnError); nackErr != nil {
				err = fmt.Errorf("%w; nack rest failed: %v", err, nackErr)
			}
		}
	}()

	successCount, handleErr := handler(ctx, bodies)
	if successCount < 0 || successCount > len(messages) {
		successCount = 0
		handleErr = fmt.Errorf("invalid batch success count")
	}
	if handleErr != nil {
		if ackErr := ackPrefix(messages[:successCount]); ackErr != nil {
			return fmt.Errorf("%w; ack prefix failed: %v", handleErr, ackErr)
		}
		if nackErr := nackRest(messages[successCount:], c.RequeueOnError); nackErr != nil {
			return fmt.Errorf("%w; nack rest failed: %v", handleErr, nackErr)
		}
		return handleErr
	}
	if successCount != len(messages) {
		if nackErr := nackRest(messages, c.RequeueOnError); nackErr != nil {
			return fmt.Errorf("batch handler returned partial success without error; nack rest failed: %v", nackErr)
		}
		return fmt.Errorf("batch handler returned partial success without error")
	}
	return ackPrefix(messages)
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

func ackPrefix(messages []IncomingMessage) error {
	for _, msg := range messages {
		if err := msg.Ack(false); err != nil {
			return err
		}
	}
	return nil
}
