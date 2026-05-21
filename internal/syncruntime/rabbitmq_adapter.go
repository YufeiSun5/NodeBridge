package syncruntime

import (
	"context"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/rabbitmq/amqp091-go"
)

type AMQPGetSource struct {
	Channel *amqp091.Channel
	Queue   string
}

func (s AMQPGetSource) Get(ctx context.Context) (rabbitmq.IncomingMessage, bool, error) {
	msg, ok, err := rabbitmq.GetOnce(ctx, s.Channel, s.Queue)
	if err != nil {
		return nil, false, err
	}
	return msg, ok, nil
}

type RoutingDownlinkDispatcher struct {
	Publisher EventPublisher
	Exchange  string
}

func (d RoutingDownlinkDispatcher) Dispatch(ctx context.Context, evt event.SyncEvent, targetNodeID string) error {
	body, err := rabbitmq.EncodeJSON(evt)
	if err != nil {
		return err
	}
	return d.Publisher.Publish(ctx, rabbitmq.PublishRequest{
		Exchange:   d.Exchange,
		RoutingKey: targetNodeID + ".downlink",
		Body:       body,
	})
}
