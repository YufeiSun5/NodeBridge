package rabbitmq

import (
	"context"
	"errors"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

type QueueGetter interface {
	Get(queue string, autoAck bool) (amqp091.Delivery, bool, error)
}

type PeekedMessage struct {
	Body        []byte
	ContentType string
	Headers     amqp091.Table
}

func PeekMessages(ctx context.Context, getter QueueGetter, queueName string, limit int) (result []PeekedMessage, err error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	result = make([]PeekedMessage, 0, limit)
	held := make([]amqp091.Delivery, 0, limit)
	defer func() {
		for _, delivery := range held {
			if nackErr := delivery.Nack(false, true); nackErr != nil {
				err = errors.Join(err, fmt.Errorf("requeue peeked message from %s: %w", queueName, nackErr))
			}
		}
	}()
	for len(result) < limit {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		delivery, ok, err := getter.Get(queueName, false)
		if err != nil {
			return result, fmt.Errorf("peek message from %s: %w", queueName, err)
		}
		if !ok {
			return result, nil
		}
		held = append(held, delivery)
		result = append(result, PeekedMessage{
			Body:        append([]byte(nil), delivery.Body...),
			ContentType: delivery.ContentType,
			Headers:     delivery.Headers,
		})
	}
	return result, nil
}
