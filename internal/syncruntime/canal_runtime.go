package syncruntime

import (
	"context"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type CanalBatchSource interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	FetchChangesOnce(ctx context.Context) ([]cdc.ChangeEvent, cdc.Offset, error)
	Commit(ctx context.Context, offset cdc.Offset) error
}

type CanalUploadRuntime struct {
	Source     CanalBatchSource
	Decider    UploadDecider
	Normalizer ChangeNormalizer
	Publisher  EventPublisher
	Exchange   string
	RoutingKey string
	started    bool
}

type ServerCanalDispatchRuntime struct {
	Source     CanalBatchSource
	Decider    UploadDecider
	Normalizer ChangeNormalizer
	Rules      *rules.RuleSet
	Dispatcher DownlinkDispatcher
	EdgeNodes  []string
	NodeStore  ActiveNodeStore
	EventStore EventLogStore
	started    bool
}

func (r *CanalUploadRuntime) RunOnce(ctx context.Context) (StepResult, error) {
	if r.Source == nil {
		return StepResult{}, fmt.Errorf("canal source is required")
	}
	if r.Normalizer == nil {
		return StepResult{}, fmt.Errorf("change normalizer is required")
	}
	if r.Publisher == nil {
		return StepResult{}, fmt.Errorf("event publisher is required")
	}
	if !r.started {
		if err := r.Source.Start(ctx); err != nil {
			return StepResult{}, err
		}
		r.started = true
	}

	changes, offset, err := r.Source.FetchChangesOnce(ctx)
	if err != nil {
		return StepResult{}, err
	}
	if len(changes) == 0 {
		return StepResult{Action: "empty"}, nil
	}

	var lastEventID string
	published := 0
	for _, change := range changes {
		if r.Decider != nil {
			decision := r.Decider.ShouldUpload(change)
			if !decision.Upload {
				continue
			}
		}
		evt, err := r.Normalizer.Normalize(change)
		if err != nil {
			return StepResult{Processed: true, EventID: lastEventID, Action: "failed"}, err
		}
		body, err := rabbitmq.EncodeJSON(evt)
		if err != nil {
			return StepResult{Processed: true, EventID: evt.EventID, Action: "failed"}, err
		}
		// ACK after publish. / 发布后 ACK。 / Publish 後 ACK。
		if err := r.Publisher.Publish(ctx, rabbitmq.PublishRequest{
			Exchange:   r.Exchange,
			RoutingKey: r.RoutingKey,
			Body:       body,
		}); err != nil {
			return StepResult{Processed: true, EventID: evt.EventID, Action: "failed"}, fmt.Errorf("publish canal event: %w", err)
		}
		lastEventID = evt.EventID
		published++
	}
	if published == 0 {
		if err := r.Source.Commit(ctx, offset); err != nil {
			return StepResult{Processed: true, Action: "failed"}, err
		}
		return StepResult{Processed: true, Action: "suppressed"}, nil
	}
	if err := r.Source.Commit(ctx, offset); err != nil {
		return StepResult{Processed: true, EventID: lastEventID, Action: "failed"}, err
	}
	return StepResult{Processed: true, EventID: lastEventID, Action: "published", DispatchCount: published}, nil
}

func (r *CanalUploadRuntime) Stop(ctx context.Context) error {
	if r.Source == nil || !r.started {
		return nil
	}
	r.started = false
	return r.Source.Stop(ctx)
}

func (r *ServerCanalDispatchRuntime) RunOnce(ctx context.Context) (StepResult, error) {
	if r.Source == nil {
		return StepResult{}, fmt.Errorf("canal source is required")
	}
	if !r.started {
		if err := r.Source.Start(ctx); err != nil {
			return StepResult{}, err
		}
		r.started = true
	}
	changes, offset, err := r.Source.FetchChangesOnce(ctx)
	if err != nil {
		return StepResult{}, err
	}
	if len(changes) == 0 {
		return StepResult{Action: "empty"}, nil
	}
	var lastEventID string
	dispatchTotal := 0
	for _, change := range changes {
		eventID, count, _, err := dispatchServerChange(ctx, change, r.Decider, r.Normalizer, r.Rules, r.Dispatcher, r.EdgeNodes, r.NodeStore, r.EventStore)
		if err != nil {
			return StepResult{Processed: true, EventID: lastEventID, Action: "failed", DispatchCount: dispatchTotal}, err
		}
		if eventID != "" {
			lastEventID = eventID
		}
		dispatchTotal += count
	}
	if err := r.Source.Commit(ctx, offset); err != nil {
		return StepResult{Processed: true, EventID: lastEventID, Action: "failed", DispatchCount: dispatchTotal}, err
	}
	if dispatchTotal == 0 {
		return StepResult{Processed: true, EventID: lastEventID, Action: "suppressed"}, nil
	}
	return StepResult{Processed: true, EventID: lastEventID, Action: "dispatched", DispatchCount: dispatchTotal}, nil
}

func (r *ServerCanalDispatchRuntime) Stop(ctx context.Context) error {
	if r.Source == nil || !r.started {
		return nil
	}
	r.started = false
	return r.Source.Stop(ctx)
}
