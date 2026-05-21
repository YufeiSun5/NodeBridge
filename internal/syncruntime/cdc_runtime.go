package syncruntime

import (
	"context"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
)

type ChangeSource interface {
	GetChange(ctx context.Context) (cdc.ChangeEvent, bool, error)
}

type ChangeNormalizer interface {
	Normalize(change cdc.ChangeEvent) (event.SyncEvent, error)
}

type UploadDecider interface {
	ShouldUpload(change cdc.ChangeEvent) loop.Decision
}

type CDCUploadRuntime struct {
	Source     ChangeSource
	Decider    UploadDecider
	Normalizer ChangeNormalizer
	Publisher  EventPublisher
	Exchange   string
	RoutingKey string
}

func (r CDCUploadRuntime) RunOnce(ctx context.Context) (StepResult, error) {
	if r.Source == nil {
		return StepResult{}, fmt.Errorf("change source is required")
	}
	if r.Normalizer == nil {
		return StepResult{}, fmt.Errorf("change normalizer is required")
	}
	if r.Publisher == nil {
		return StepResult{}, fmt.Errorf("event publisher is required")
	}

	change, ok, err := r.Source.GetChange(ctx)
	if err != nil {
		return StepResult{}, err
	}
	if !ok {
		return StepResult{Action: "empty"}, nil
	}

	if r.Decider != nil {
		decision := r.Decider.ShouldUpload(change)
		if !decision.Upload {
			return StepResult{Processed: true, Action: "suppressed"}, nil
		}
	}

	evt, err := r.Normalizer.Normalize(change)
	if err != nil {
		return StepResult{Processed: true, Action: "failed"}, err
	}
	body, err := rabbitmq.EncodeJSON(evt)
	if err != nil {
		return StepResult{Processed: true, EventID: evt.EventID, Action: "failed"}, err
	}
	if err := r.Publisher.Publish(ctx, rabbitmq.PublishRequest{
		Exchange:   r.Exchange,
		RoutingKey: r.RoutingKey,
		Body:       body,
	}); err != nil {
		return StepResult{Processed: true, EventID: evt.EventID, Action: "failed"}, fmt.Errorf("publish cdc event: %w", err)
	}

	return StepResult{Processed: true, EventID: evt.EventID, Action: "published"}, nil
}
