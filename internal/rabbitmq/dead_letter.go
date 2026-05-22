package rabbitmq

import (
	"context"
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

func PeekMessages(ctx context.Context, getter QueueGetter, queueName string, limit int) ([]PeekedMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	result := make([]PeekedMessage, 0, limit)
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
		result = append(result, PeekedMessage{
			Body:        append([]byte(nil), delivery.Body...),
			ContentType: delivery.ContentType,
			Headers:     delivery.Headers,
		})
		// Peek, then return. / 只看后放回。 / 見たら戻す。
		if err := delivery.Nack(false, true); err != nil {
			return result, fmt.Errorf("requeue peeked message from %s: %w", queueName, err)
		}
	}
	return result, nil
}
