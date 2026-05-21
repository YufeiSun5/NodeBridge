package rabbitmq

import (
	"encoding/json"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

type DeliveryMessage struct {
	Delivery amqp091.Delivery
}

func (m DeliveryMessage) Body() []byte {
	return m.Delivery.Body
}

func (m DeliveryMessage) Ack(multiple bool) error {
	return m.Delivery.Ack(multiple)
}

func (m DeliveryMessage) Nack(multiple, requeue bool) error {
	return m.Delivery.Nack(multiple, requeue)
}

func EncodeJSON(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode json message: %w", err)
	}
	return body, nil
}
