package rabbitmq

import (
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

type Declarer interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp091.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp091.Table) (amqp091.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp091.Table) error
}

func InitializeTopology(declarer Declarer, topology Topology) error {
	for _, exchange := range topology.Exchanges {
		if err := declarer.ExchangeDeclare(
			exchange.Name,
			exchange.Kind,
			exchange.Durable,
			exchange.AutoDelete,
			exchange.Internal,
			exchange.NoWait,
			exchange.Args,
		); err != nil {
			return fmt.Errorf("declare exchange %s: %w", exchange.Name, err)
		}
	}

	for _, queue := range topology.Queues {
		if _, err := declarer.QueueDeclare(
			queue.Name,
			queue.Durable,
			queue.AutoDelete,
			queue.Exclusive,
			queue.NoWait,
			queue.Args,
		); err != nil {
			return fmt.Errorf("declare queue %s: %w", queue.Name, err)
		}
	}

	for _, binding := range topology.Bindings {
		if err := declarer.QueueBind(
			binding.Queue,
			binding.RoutingKey,
			binding.Exchange,
			binding.NoWait,
			binding.Args,
		); err != nil {
			return fmt.Errorf("bind queue %s to %s: %w", binding.Queue, binding.Exchange, err)
		}
	}

	return nil
}
