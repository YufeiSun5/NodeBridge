package syncruntime

import (
	"context"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
)

type ChangeSource interface {
	GetChange(ctx context.Context) (cdc.ChangeEvent, bool, error)
}

type ChangeNormalizer interface {
	Normalize(change cdc.ChangeEvent) (event.SyncEvent, error)
}

type UploadDecider interface {
	ShouldUpload(ctx context.Context, change cdc.ChangeEvent) (loop.Decision, error)
}

type CDCUploadRuntime struct {
	Source     ChangeSource
	Decider    UploadDecider
	Normalizer ChangeNormalizer
	Publisher  EventPublisher
	Exchange   string
	RoutingKey string
}

type ServerCDCDispatchRuntime struct {
	Source     ChangeSource
	Decider    UploadDecider
	Normalizer ChangeNormalizer
	Rules      *rules.RuleSet
	Dispatcher DownlinkDispatcher
	EdgeNodes  []string
	NodeStore  ActiveNodeStore
	EventStore EventLogStore
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
		decision, err := r.Decider.ShouldUpload(ctx, change)
		if err != nil {
			return StepResult{Processed: true, Action: "failed"}, err
		}
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

func (r ServerCDCDispatchRuntime) RunOnce(ctx context.Context) (StepResult, error) {
	if r.Source == nil {
		return StepResult{}, fmt.Errorf("change source is required")
	}
	change, ok, err := r.Source.GetChange(ctx)
	if err != nil {
		return StepResult{}, err
	}
	if !ok {
		return StepResult{Action: "empty"}, nil
	}
	eventID, dispatchCount, action, err := dispatchServerChange(ctx, change, r.Decider, r.Normalizer, r.Rules, r.Dispatcher, r.EdgeNodes, r.NodeStore, r.EventStore)
	if err != nil {
		return StepResult{Processed: true, EventID: eventID, Action: "failed"}, err
	}
	return StepResult{Processed: true, EventID: eventID, Action: action, DispatchCount: dispatchCount}, nil
}

func dispatchServerChange(ctx context.Context, change cdc.ChangeEvent, decider UploadDecider, normalizer ChangeNormalizer, ruleSet *rules.RuleSet, dispatcher DownlinkDispatcher, edgeNodes []string, nodeStore ActiveNodeStore, eventStore EventLogStore) (string, int, string, error) {
	if dispatcher == nil {
		return "", 0, "", fmt.Errorf("downlink dispatcher is required")
	}
	record, rule, err := prepareServerChange(ctx, change, decider, normalizer, ruleSet)
	eventID := record.Event.EventID
	if err != nil {
		return eventID, 0, "", err
	}
	if rule == nil {
		return eventID, 0, "suppressed", nil
	}
	if eventStore != nil {
		if err := eventStore.UpsertEventLog(ctx, record); err != nil {
			return eventID, 0, "", fmt.Errorf("persist server cdc event: %w", err)
		}
	}
	count, err := (ServerIngressRuntime{Dispatcher: dispatcher, EdgeNodes: edgeNodes, NodeStore: nodeStore}).dispatch(ctx, record.Event, *rule)
	if err != nil {
		return eventID, count, "", err
	}
	if eventStore != nil {
		if err := completeEventLogs(ctx, eventStore, []syncstore.EventLogRecord{record}); err != nil {
			return eventID, count, "", fmt.Errorf("persist dispatched server cdc event: %w", err)
		}
	}
	return eventID, count, "dispatched", nil
}

func prepareServerChange(ctx context.Context, change cdc.ChangeEvent, decider UploadDecider, normalizer ChangeNormalizer, ruleSet *rules.RuleSet) (syncstore.EventLogRecord, *rules.SyncRule, error) {
	defer measurePhase(ctx, "cdc_prepare")()
	var record syncstore.EventLogRecord
	if normalizer == nil {
		return record, nil, fmt.Errorf("change normalizer is required")
	}
	if ruleSet == nil {
		return record, nil, fmt.Errorf("rules are required")
	}
	if decider != nil {
		decision, err := decider.ShouldUpload(ctx, change)
		if err != nil {
			return record, nil, err
		}
		if !decision.Upload {
			return record, nil, nil
		}
	}
	evt, err := normalizer.Normalize(change)
	if err != nil {
		return record, nil, err
	}
	record.Event = evt
	rule := findRuleForEvent(ruleSet, evt)
	if rule == nil || !rule.Enable || rule.Direction == rules.DirectionIgnore || !shouldDispatch(*rule) {
		return record, nil, nil
	}
	mapped, err := mapper.MapEvent(evt, *rule)
	if err != nil {
		return record, nil, fmt.Errorf("map server cdc event: %w", err)
	}
	if !schemaChangeAllowed(evt, mapped, *rule) {
		return record, nil, nil
	}
	body, err := rabbitmq.EncodeJSON(evt)
	if err != nil {
		return record, nil, err
	}
	record.TargetDatabaseName, record.TargetTableName = mapped.TargetDatabase, mapped.TargetTable
	record.PKValue, record.Direction = pkValue(evt.PrimaryKey), rule.Direction
	record.Status, record.Payload = syncstore.StatusPending, body
	return record, rule, nil
}
