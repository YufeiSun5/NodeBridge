package syncruntime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type MessageSource interface {
	Get(ctx context.Context) (rabbitmq.IncomingMessage, bool, error)
}

type EventPublisher interface {
	Publish(ctx context.Context, req rabbitmq.PublishRequest) error
}

type DownlinkDispatcher interface {
	Dispatch(ctx context.Context, evt event.SyncEvent, targetNodeID string) error
}

type StepResult struct {
	Processed     bool
	EventID       string
	Action        string
	DispatchCount int
}

type EdgeUploadRuntime struct {
	Source     MessageSource
	Publisher  EventPublisher
	Consumer   rabbitmq.Consumer
	Exchange   string
	RoutingKey string
}

func (r EdgeUploadRuntime) RunOnce(ctx context.Context) (StepResult, error) {
	if r.Source == nil {
		return StepResult{}, fmt.Errorf("message source is required")
	}
	if r.Publisher == nil {
		return StepResult{}, fmt.Errorf("event publisher is required")
	}

	msg, ok, err := r.Source.Get(ctx)
	if err != nil {
		return StepResult{}, err
	}
	if !ok {
		return StepResult{Action: "empty"}, nil
	}

	var eventID string
	// ACK after apply. / Apply 后 ACK。 / Apply 後 ACK。
	err = r.Consumer.Handle(ctx, msg, func(ctx context.Context, body []byte) error {
		var evt event.SyncEvent
		if err := json.Unmarshal(body, &evt); err != nil {
			return fmt.Errorf("parse upload event: %w", err)
		}
		eventID = evt.EventID
		if err := r.Publisher.Publish(ctx, rabbitmq.PublishRequest{
			Exchange:   r.Exchange,
			RoutingKey: r.RoutingKey,
			Body:       body,
		}); err != nil {
			return fmt.Errorf("forward upload event: %w", err)
		}
		return nil
	})
	if err != nil {
		return StepResult{Processed: true, EventID: eventID, Action: "failed"}, err
	}
	return StepResult{Processed: true, EventID: eventID, Action: "forwarded"}, nil
}

type ServerIngressRuntime struct {
	Source     MessageSource
	Consumer   rabbitmq.Consumer
	Rules      *rules.RuleSet
	Worker     apply.Worker
	Dispatcher DownlinkDispatcher
	EdgeNodes  []string
}

type EdgeDownlinkRuntime struct {
	Source   MessageSource
	Consumer rabbitmq.Consumer
	Rules    *rules.RuleSet
	Worker   apply.Worker
}

func (r EdgeDownlinkRuntime) RunOnce(ctx context.Context) (StepResult, error) {
	if r.Source == nil {
		return StepResult{}, fmt.Errorf("message source is required")
	}
	if r.Rules == nil {
		return StepResult{}, fmt.Errorf("rules are required")
	}
	if r.Worker == nil {
		return StepResult{}, fmt.Errorf("apply worker is required")
	}

	msg, ok, err := r.Source.Get(ctx)
	if err != nil {
		return StepResult{}, err
	}
	if !ok {
		return StepResult{Action: "empty"}, nil
	}

	var eventID string
	// ACK after apply. / Apply 后 ACK。 / Apply 後 ACK。
	err = r.Consumer.Handle(ctx, msg, func(ctx context.Context, body []byte) error {
		evt, mapped, err := mapSyncEvent(body, r.Rules)
		if err != nil {
			return err
		}
		eventID = evt.EventID
		rule := r.Rules.Find(evt.DatabaseName, evt.TableName)
		if !rule.Enable || rule.Direction == rules.DirectionIgnore {
			return nil
		}
		if _, err := r.Worker.Apply(ctx, mapped); err != nil {
			return fmt.Errorf("apply downlink event: %w", err)
		}
		return nil
	})
	if err != nil {
		return StepResult{Processed: true, EventID: eventID, Action: "failed"}, err
	}
	return StepResult{Processed: true, EventID: eventID, Action: "applied"}, nil
}

func (r ServerIngressRuntime) RunOnce(ctx context.Context) (StepResult, error) {
	if r.Source == nil {
		return StepResult{}, fmt.Errorf("message source is required")
	}
	if r.Rules == nil {
		return StepResult{}, fmt.Errorf("rules are required")
	}
	if r.Worker == nil {
		return StepResult{}, fmt.Errorf("apply worker is required")
	}

	msg, ok, err := r.Source.Get(ctx)
	if err != nil {
		return StepResult{}, err
	}
	if !ok {
		return StepResult{Action: "empty"}, nil
	}

	var eventID string
	var dispatchCount int
	err = r.Consumer.Handle(ctx, msg, func(ctx context.Context, body []byte) error {
		evt, mapped, err := mapSyncEvent(body, r.Rules)
		if err != nil {
			return err
		}
		eventID = evt.EventID
		rule := r.Rules.Find(evt.DatabaseName, evt.TableName)
		if !rule.Enable || rule.Direction == rules.DirectionIgnore {
			return nil
		}
		if _, err := r.Worker.Apply(ctx, mapped); err != nil {
			return fmt.Errorf("apply ingress event: %w", err)
		}
		if shouldDispatch(*rule) {
			count, err := r.dispatch(ctx, evt)
			if err != nil {
				return err
			}
			dispatchCount = count
		}
		return nil
	})
	if err != nil {
		return StepResult{Processed: true, EventID: eventID, Action: "failed"}, err
	}
	return StepResult{Processed: true, EventID: eventID, Action: "applied", DispatchCount: dispatchCount}, nil
}

func (r ServerIngressRuntime) dispatch(ctx context.Context, evt event.SyncEvent) (int, error) {
	if r.Dispatcher == nil {
		return 0, nil
	}
	count := 0
	for _, nodeID := range r.EdgeNodes {
		if nodeID == "" || nodeID == evt.OriginNodeID {
			continue
		}
		if err := r.Dispatcher.Dispatch(ctx, evt, nodeID); err != nil {
			return count, fmt.Errorf("dispatch event %s to %s: %w", evt.EventID, nodeID, err)
		}
		count++
	}
	return count, nil
}

func shouldDispatch(rule rules.SyncRule) bool {
	return rule.Direction == rules.DirectionBidirectional || rule.Direction == rules.DirectionServerToEdge
}

func mapSyncEvent(body []byte, ruleSet *rules.RuleSet) (event.SyncEvent, mapper.MappedEvent, error) {
	var evt event.SyncEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return event.SyncEvent{}, mapper.MappedEvent{}, fmt.Errorf("parse sync event: %w", err)
	}
	rule := ruleSet.Find(evt.DatabaseName, evt.TableName)
	if rule == nil {
		return event.SyncEvent{}, mapper.MappedEvent{}, fmt.Errorf("sync rule not found for %s.%s", evt.DatabaseName, evt.TableName)
	}
	if !rule.Enable || rule.Direction == rules.DirectionIgnore {
		return evt, mapper.MappedEvent{}, nil
	}
	mapped, err := mapper.MapEvent(evt, *rule)
	if err != nil {
		return event.SyncEvent{}, mapper.MappedEvent{}, fmt.Errorf("map sync event: %w", err)
	}
	return evt, mapped, nil
}
