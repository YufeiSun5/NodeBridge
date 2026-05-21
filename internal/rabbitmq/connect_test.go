package rabbitmq_test

import (
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/rabbitmq/amqp091-go"
)

func TestDeliveryMessage(t *testing.T) {
	msg := rabbitmq.DeliveryMessage{Delivery: amqp091.Delivery{Body: []byte("payload")}}

	if string(msg.Body()) != "payload" {
		t.Fatalf("unexpected body %q", msg.Body())
	}
}
