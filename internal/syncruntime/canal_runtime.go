package syncruntime

import (
	"context"
	"fmt"
	"strconv"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
)

type CanalBatchSource interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	FetchChangesOnce(ctx context.Context) ([]cdc.ChangeEvent, cdc.Offset, error)
	Commit(ctx context.Context, offset cdc.Offset) error
}

// LocalVersionRecorder persists source versions without locking business rows.
// It must be idempotent: a failed publish or checkpoint replays the entire batch.
type LocalVersionRecorder interface {
	RecordLocal(context.Context, event.SyncEvent) error
}

type CanalUploadRuntime struct {
	LocalVersions LocalVersionRecorder
	Source        CanalBatchSource
	Decider       UploadDecider
	Normalizer    ChangeNormalizer
	Publisher     EventPublisher
	Exchange      string
	RoutingKey    string
	started       bool
}

type ServerCanalDispatchRuntime struct {
	LocalVersions LocalVersionRecorder
	Source        CanalBatchSource
	Decider       UploadDecider
	Normalizer    ChangeNormalizer
	Rules         *rules.RuleSet
	Dispatcher    DownlinkDispatcher
	EdgeNodes     []string
	NodeStore     ActiveNodeStore
	EventStore    EventLogStore
	started       bool
}

func (r *CanalUploadRuntime) RunOnce(ctx context.Context) (result StepResult, err error) {
	if r.Source == nil {
		return StepResult{}, fmt.Errorf("canal source is required")
	}
	defer func() {
		if err != nil {
			r.resetSource()
		}
	}()
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

	doneFetch := measurePhase(ctx, "cdc_fetch")
	changes, offset, err := r.Source.FetchChangesOnce(ctx)
	doneFetch()
	if err != nil {
		return StepResult{}, err
	}
	if len(changes) == 0 {
		if offset.HasCanalBatch() {
			offset.SkipCheckpoint = true
			if err := commitCanalBatch(ctx, r.Source, offset); err != nil {
				return StepResult{Processed: true, Action: "failed"}, err
			}
			return StepResult{Processed: true, Action: "committed-empty"}, nil
		}
		return StepResult{Action: "empty"}, nil
	}

	var lastEventID string
	requests := make([]rabbitmq.PublishRequest, 0, len(changes))
	for _, change := range changes {
		if err := ctx.Err(); err != nil {
			return StepResult{Processed: true, EventID: lastEventID, Action: "failed"}, err
		}
		if r.Decider != nil {
			done := measurePhase(ctx, "replay_check")
			decision, err := r.Decider.ShouldUpload(ctx, change)
			done()
			if err != nil {
				return StepResult{Processed: true, EventID: lastEventID, Action: "failed"}, err
			}
			if !decision.Upload {
				continue
			}
		}
		evt, err := r.Normalizer.Normalize(change)
		if err != nil {
			return StepResult{Processed: true, EventID: lastEventID, Action: "failed"}, err
		}
		if r.LocalVersions != nil {
			if err := r.LocalVersions.RecordLocal(ctx, evt); err != nil {
				return StepResult{Processed: true, EventID: evt.EventID, Action: "failed"}, fmt.Errorf("record local version: %w", err)
			}
		}
		body, err := rabbitmq.EncodeJSON(evt)
		if err != nil {
			return StepResult{Processed: true, EventID: evt.EventID, Action: "failed"}, err
		}
		requests = append(requests, rabbitmq.PublishRequest{
			Exchange:   r.Exchange,
			RoutingKey: r.RoutingKey,
			Body:       body,
		})
		lastEventID = evt.EventID
	}
	if len(requests) == 0 {
		offset.SkipCheckpoint = true
		if err := commitCanalBatch(ctx, r.Source, offset); err != nil {
			return StepResult{Processed: true, Action: "failed"}, err
		}
		return StepResult{Processed: true, Action: "suppressed"}, nil
	}
	// ACK after publish. / 发布后 ACK。 / Publish 後 ACK。
	if err := publishCanalBatch(ctx, r.Publisher, requests); err != nil {
		return StepResult{Processed: true, EventID: lastEventID, Action: "failed"}, fmt.Errorf("publish canal event: %w", err)
	}
	if err := commitCanalBatch(ctx, r.Source, offset); err != nil {
		return StepResult{Processed: true, EventID: lastEventID, Action: "failed"}, err
	}
	return StepResult{Processed: true, EventID: lastEventID, Action: "published", DispatchCount: len(requests)}, nil
}

func (r *CanalUploadRuntime) Stop(ctx context.Context) error {
	if r.Source == nil || !r.started {
		return nil
	}
	r.started = false
	return r.Source.Stop(ctx)
}

func (r *CanalUploadRuntime) resetSource() {
	if r.Source == nil || !r.started {
		return
	}
	r.started = false
	_ = r.Source.Stop(context.Background())
}

func publishCanalBatch(ctx context.Context, publisher EventPublisher, requests []rabbitmq.PublishRequest) error {
	defer measurePhase(ctx, "publish_confirm")()
	if batchPublisher, ok := publisher.(BatchEventPublisher); ok {
		return batchPublisher.PublishBatch(ctx, requests)
	}
	for _, req := range requests {
		if err := publisher.Publish(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

func commitCanalBatch(ctx context.Context, source CanalBatchSource, offset cdc.Offset) error {
	defer measurePhase(ctx, "cdc_checkpoint")()
	return source.Commit(ctx, offset)
}

func (r *ServerCanalDispatchRuntime) RunOnce(ctx context.Context) (result StepResult, err error) {
	if r.Source == nil {
		return StepResult{}, fmt.Errorf("canal source is required")
	}
	// Reconnect rolls back unacknowledged batches before any later fetch.
	defer func() {
		if err != nil {
			r.resetSource()
		}
	}()
	if !r.started {
		if err := r.Source.Start(ctx); err != nil {
			return StepResult{}, err
		}
		r.started = true
	}
	doneFetch := measurePhase(ctx, "cdc_fetch")
	changes, offset, err := r.Source.FetchChangesOnce(ctx)
	doneFetch()
	if err != nil {
		return StepResult{}, err
	}
	if len(changes) == 0 {
		if offset.HasCanalBatch() {
			offset.SkipCheckpoint = true
			if err := commitCanalBatch(ctx, r.Source, offset); err != nil {
				return StepResult{Processed: true, Action: "failed"}, err
			}
			return StepResult{Processed: true, Action: "committed-empty"}, nil
		}
		return StepResult{Action: "empty"}, nil
	}
	lastEventID, dispatchTotal, err := r.dispatchChanges(ctx, changes)
	if err != nil {
		return StepResult{Processed: true, EventID: lastEventID, Action: "failed", DispatchCount: dispatchTotal}, err
	}
	if err := ctx.Err(); err != nil {
		return StepResult{Processed: true, EventID: lastEventID, Action: "failed", DispatchCount: dispatchTotal}, err
	}
	// ACK-only batches must not feed their own SQL checkpoint writes back into CDC.
	offset.SkipCheckpoint = lastEventID == ""
	if err := commitCanalBatch(ctx, r.Source, offset); err != nil {
		return StepResult{Processed: true, EventID: lastEventID, Action: "failed", DispatchCount: dispatchTotal}, err
	}
	if dispatchTotal == 0 {
		return StepResult{Processed: true, EventID: lastEventID, Action: "suppressed"}, nil
	}
	return StepResult{Processed: true, EventID: lastEventID, Action: "dispatched", DispatchCount: dispatchTotal}, nil
}

func (r *ServerCanalDispatchRuntime) dispatchChanges(ctx context.Context, changes []cdc.ChangeEvent) (string, int, error) {
	if r.Dispatcher == nil {
		return "", 0, fmt.Errorf("downlink dispatcher is required")
	}
	var lastEventID string
	var activeNodes []string
	activeNodesResolved := false
	records := make([]syncstore.EventLogRecord, 0, len(changes))
	var batches [][]DownlinkRequest
	batchKind := ""
	updateKeys := map[string]bool{}
	for _, change := range changes {
		if err := ctx.Err(); err != nil {
			return lastEventID, 0, err
		}
		record, rule, err := prepareServerChange(ctx, change, r.Decider, r.Normalizer, r.Rules)
		if record.Event.EventID != "" {
			lastEventID = record.Event.EventID
		}
		if err != nil {
			return lastEventID, 0, err
		}
		if rule == nil {
			continue
		}
		if r.LocalVersions != nil {
			if err := r.LocalVersions.RecordLocal(ctx, record.Event); err != nil {
				return lastEventID, 0, fmt.Errorf("record local version: %w", err)
			}
		}
		nodeIDs := rule.DispatchNodeIDs
		if dispatchTarget(*rule) == rules.DispatchActiveEdges {
			if !activeNodesResolved {
				activeNodes, err = dispatchNodeIDs(ctx, *rule, r.EdgeNodes, r.NodeStore)
				if err != nil {
					return lastEventID, 0, err
				}
				activeNodesResolved = true
			}
			nodeIDs = activeNodes
		}
		records = append(records, record)
		kind, key := "", ""
		if rule.SyncMode == rules.SyncModeAppendOnly && record.Event.EventType == event.TypeInsert {
			kind = "append"
		} else if record.Event.EventType == event.TypeUpdate && rule.SyncMode != rules.SyncModeAppendOnly {
			mapped, err := mapper.MapEvent(record.Event, *rule)
			if err != nil {
				return lastEventID, 0, err
			}
			if candidate, safe := independentUpdateKey(mapped); safe {
				kind = "update|" + rule.ID + "|" + mapped.TargetDatabase + "." + mapped.TargetTable
				key = candidate
			}
		}
		// A repeated key and every non-UPDATE CRUD/DDL operation are confirmation barriers.
		if kind == "" || kind != batchKind || (key != "" && (updateKeys[key] || len(updateKeys) >= 64)) {
			batches = append(batches, nil)
			updateKeys = map[string]bool{}
		}
		batchKind = kind
		if key != "" {
			updateKeys[key] = true
		}
		for _, nodeID := range nodeIDs {
			if nodeID != "" && nodeID != record.Event.OriginNodeID {
				index := len(batches) - 1
				batches[index] = append(batches[index], DownlinkRequest{Event: record.Event, TargetNodeID: nodeID})
			}
		}
	}
	if r.EventStore != nil {
		if err := upsertEventLogs(ctx, r.EventStore, records); err != nil {
			return lastEventID, 0, fmt.Errorf("persist server cdc batch: %w", err)
		}
	}
	count := 0
	for _, batch := range batches {
		sent, err := dispatchDownlinkBatch(ctx, r.Dispatcher, batch)
		count += sent
		if err != nil {
			return lastEventID, count, err
		}
	}
	if r.EventStore != nil {
		if err := completeEventLogs(ctx, r.EventStore, records); err != nil {
			return lastEventID, count, fmt.Errorf("persist dispatched server cdc batch: %w", err)
		}
	}
	return lastEventID, count, nil
}

func independentUpdateKey(mapped mapper.MappedEvent) (string, bool) {
	if len(mapped.TargetPrimaryKey) == 0 {
		return "", false
	}
	for column, value := range mapped.TargetPrimaryKey {
		text := fmt.Sprint(value)
		integer, signedErr := strconv.ParseInt(text, 10, 64)
		canonical := signedErr == nil && strconv.FormatInt(integer, 10) == text
		if !canonical {
			unsigned, err := strconv.ParseUint(text, 10, 64)
			canonical = err == nil && strconv.FormatUint(unsigned, 10) == text
		}
		// Unknown text collations and alternate numeric spellings must remain serial.
		if !canonical {
			return "", false
		}
		if before, ok := mapped.TargetBefore[column]; ok && fmt.Sprint(before) != text {
			return "", false
		}
		if after, ok := mapped.TargetAfter[column]; ok && fmt.Sprint(after) != text {
			return "", false
		}
	}
	return pkValue(mapped.TargetPrimaryKey), true
}

func dispatchDownlinkBatch(ctx context.Context, dispatcher DownlinkDispatcher, requests []DownlinkRequest) (int, error) {
	defer measurePhase(ctx, "publish_confirm")()
	if len(requests) == 0 {
		return 0, nil
	}
	if batchDispatcher, ok := dispatcher.(BatchDownlinkDispatcher); ok {
		if err := batchDispatcher.DispatchBatch(ctx, requests); err != nil {
			return 0, fmt.Errorf("dispatch server cdc batch: %w", err)
		}
		return len(requests), nil
	}
	for i, request := range requests {
		if err := dispatcher.Dispatch(ctx, request.Event, request.TargetNodeID); err != nil {
			return i, fmt.Errorf("dispatch event %s to %s: %w", request.Event.EventID, request.TargetNodeID, err)
		}
	}
	return len(requests), nil
}

func (r *ServerCanalDispatchRuntime) Stop(ctx context.Context) error {
	if r.Source == nil || !r.started {
		return nil
	}
	r.started = false
	return r.Source.Stop(ctx)
}

func (r *ServerCanalDispatchRuntime) resetSource() {
	if r.Source == nil || !r.started {
		return
	}
	r.started = false
	_ = r.Source.Stop(context.Background())
}
