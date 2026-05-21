package rabbitmq

import (
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

type QueueInspector interface {
	QueueDeclarePassive(name string, durable, autoDelete, exclusive, noWait bool, args amqp091.Table) (amqp091.Queue, error)
}

type QueueStatus struct {
	Name      string `json:"name"`
	Messages  int    `json:"messages"`
	Consumers int    `json:"consumers"`
}

func InspectQueue(inspector QueueInspector, queue Queue) (QueueStatus, error) {
	result, err := inspector.QueueDeclarePassive(
		queue.Name,
		queue.Durable,
		queue.AutoDelete,
		queue.Exclusive,
		queue.NoWait,
		queue.Args,
	)
	if err != nil {
		return QueueStatus{}, fmt.Errorf("inspect queue %s: %w", queue.Name, err)
	}
	return QueueStatus{Name: result.Name, Messages: result.Messages, Consumers: result.Consumers}, nil
}
