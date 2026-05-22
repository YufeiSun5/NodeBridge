package syncruntime

import (
	"context"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/rabbitmq/amqp091-go"
)

const (
	DefaultBatchSize     = 50
	DefaultFlushInterval = 500 * time.Millisecond
)

type BatchMessageSource interface {
	GetBatch(ctx context.Context, max int, flushInterval time.Duration) ([]rabbitmq.IncomingMessage, error)
}

type AMQPBatchGetSource struct {
	Channel *amqp091.Channel
	Queue   string
	Sleep   func(context.Context, time.Duration) error
}

func (s AMQPBatchGetSource) GetBatch(ctx context.Context, max int, flushInterval time.Duration) ([]rabbitmq.IncomingMessage, error) {
	if max <= 0 {
		max = DefaultBatchSize
	}
	if flushInterval <= 0 {
		flushInterval = DefaultFlushInterval
	}
	sleep := s.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	deadline := time.Now().Add(flushInterval)
	messages := make([]rabbitmq.IncomingMessage, 0, max)
	for len(messages) < max {
		msg, ok, err := rabbitmq.GetOnce(ctx, s.Channel, s.Queue)
		if err != nil {
			return nil, err
		}
		if ok {
			messages = append(messages, msg)
			continue
		}
		if len(messages) > 0 || !time.Now().Before(deadline) {
			break
		}
		if err := sleep(ctx, 10*time.Millisecond); err != nil {
			return nil, err
		}
	}
	return messages, nil
}
