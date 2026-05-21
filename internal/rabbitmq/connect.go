package rabbitmq

import (
	"context"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

type Connection struct {
	Conn    *amqp091.Connection
	Channel *amqp091.Channel
}

func Dial(url string) (*Connection, error) {
	conn, err := amqp091.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}
	return &Connection{Conn: conn, Channel: ch}, nil
}

func (c *Connection) Close() error {
	if c == nil {
		return nil
	}
	var err error
	if c.Channel != nil {
		err = c.Channel.Close()
	}
	if c.Conn != nil {
		if closeErr := c.Conn.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}

func GetOnce(ctx context.Context, ch *amqp091.Channel, queueName string) (DeliveryMessage, bool, error) {
	delivery, ok, err := ch.Get(queueName, false)
	if err != nil {
		return DeliveryMessage{}, false, fmt.Errorf("get message from %s: %w", queueName, err)
	}
	return DeliveryMessage{Delivery: delivery}, ok, nil
}
