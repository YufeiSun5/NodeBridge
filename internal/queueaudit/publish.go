package queueaudit

import (
	"context"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

// PublishCopy uses a dedicated channel so returned publications cannot be
// confused with another publisher's confirmation.
func PublishCopy(ctx context.Context, connection *amqp091.Connection, queue string, delivery amqp091.Delivery, planID string) error {
	channel, err := connection.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	if _, err := channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return err
	}
	if err := channel.Confirm(false); err != nil {
		return err
	}
	confirmed := channel.NotifyPublish(make(chan amqp091.Confirmation, 1))
	returned := channel.NotifyReturn(make(chan amqp091.Return, 1))
	headers := amqp091.Table{}
	for key, value := range delivery.Headers {
		headers[key] = value
	}
	headers["nodebridge_quarantine_plan_id"] = planID
	headers["nodebridge_original_expiration"] = delivery.Expiration
	publication := amqp091.Publishing{Headers: headers, ContentType: delivery.ContentType, ContentEncoding: delivery.ContentEncoding, DeliveryMode: amqp091.Persistent, Priority: delivery.Priority, CorrelationId: delivery.CorrelationId, ReplyTo: delivery.ReplyTo, MessageId: delivery.MessageId, Timestamp: delivery.Timestamp, Type: delivery.Type, AppId: delivery.AppId, Body: delivery.Body}
	if err := channel.PublishWithContext(ctx, "", queue, true, false, publication); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case response := <-returned:
		return fmt.Errorf("quarantine publish returned: %d %s", response.ReplyCode, response.ReplyText)
	case confirmation, ok := <-confirmed:
		if !ok || !confirmation.Ack {
			return fmt.Errorf("quarantine publish not confirmed")
		}
		select {
		case response := <-returned:
			return fmt.Errorf("quarantine publish returned: %d %s", response.ReplyCode, response.ReplyText)
		default:
			return nil
		}
	}
}
