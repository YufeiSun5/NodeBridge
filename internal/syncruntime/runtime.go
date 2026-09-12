package syncruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
)

type MessageSource interface {
	Get(ctx context.Context) (rabbitmq.IncomingMessage, bool, error)
}

type EventPublisher interface {
	Publish(ctx context.Context, req rabbitmq.PublishRequest) error
}

type BatchEventPublisher interface {
	PublishBatch(ctx context.Context, reqs []rabbitmq.PublishRequest) error
}

type DownlinkDispatcher interface {
	Dispatch(ctx context.Context, evt event.SyncEvent, targetNodeID string) error
}

type DownlinkRequest struct {
	Event        event.SyncEvent
	TargetNodeID string
}

type BatchDownlinkDispatcher interface {
	DispatchBatch(ctx context.Context, requests []DownlinkRequest) error
}

type EventLogStore interface {
	UpsertEventLog(ctx context.Context, record syncstore.EventLogRecord) error
}

type EventLogBatchStore interface {
	UpsertEventLogs(ctx context.Context, records []syncstore.EventLogRecord) error
}

type EventLogSuccessStore interface {
	MarkEventLogsSucceeded(ctx context.Context, eventIDs []string, appliedAt time.Time) error
}

type NodeConfigStore interface {
	UpsertNodeConfig(ctx context.Context, config syncstore.NodeConfig) error
}

type ActiveNodeStore interface {
	ListActiveEdgeNodeIDs(ctx context.Context) ([]string, error)
}

type ReplayStore interface {
	ListPendingReplays(ctx context.Context, limit int) ([]syncstore.ReplayEvent, error)
	UpsertAck(ctx context.Context, record syncstore.AckRecord) error
	UpsertDispatch(ctx context.Context, record syncstore.DispatchRecord) error
}

type StepResult struct {
	Processed     bool
	EventID       string
	Action        string
	DispatchCount int
	Count         int
}

type EdgeUploadRuntime struct {
	Source     MessageSource
	Publisher  EventPublisher
	Consumer   rabbitmq.Consumer
	Exchange   string
	RoutingKey string
}

type EdgeUploadBatchRuntime struct {
	Source        BatchMessageSource
	Publisher     EventPublisher
	Consumer      rabbitmq.Consumer
	Exchange      string
	RoutingKey    string
	MaxBatch      int
	FlushInterval time.Duration
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
		if err := json.Unmarshal(cleanJSONBody(body), &evt); err != nil {
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

func (r EdgeUploadBatchRuntime) RunOnce(ctx context.Context) (StepResult, error) {
	if r.Source == nil {
		return StepResult{}, fmt.Errorf("batch message source is required")
	}
	if r.Publisher == nil {
		return StepResult{}, fmt.Errorf("event publisher is required")
	}
	messages, err := r.Source.GetBatch(ctx, defaultBatchSize(r.MaxBatch), defaultFlushInterval(r.FlushInterval))
	if err != nil {
		return StepResult{}, err
	}
	if len(messages) == 0 {
		return StepResult{Action: "empty"}, nil
	}

	var lastEventID string
	if batchPublisher, ok := r.Publisher.(BatchEventPublisher); ok {
		err = r.Consumer.HandleBatchCommit(ctx, messages, func(ctx context.Context, bodies [][]byte) (int, error) {
			requests := make([]rabbitmq.PublishRequest, 0, len(bodies))
			for _, body := range bodies {
				eventID, err := eventIDFromBody(body)
				if err != nil {
					return 0, err
				}
				lastEventID = eventID
				requests = append(requests, rabbitmq.PublishRequest{
					Exchange:   r.Exchange,
					RoutingKey: r.RoutingKey,
					Body:       body,
				})
			}
			if err := batchPublisher.PublishBatch(ctx, requests); err != nil {
				return 0, fmt.Errorf("forward upload batch: %w", err)
			}
			return len(bodies), nil
		})
		if err != nil {
			return StepResult{Processed: true, EventID: lastEventID, Action: "failed", Count: len(messages)}, err
		}
		return StepResult{Processed: true, EventID: lastEventID, Action: "forwarded", Count: len(messages)}, nil
	}

	err = r.Consumer.HandleBatch(ctx, messages, func(ctx context.Context, body []byte) error {
		eventID, err := eventIDFromBody(body)
		if err != nil {
			return err
		}
		lastEventID = eventID
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
		return StepResult{Processed: true, EventID: lastEventID, Action: "failed", Count: len(messages)}, err
	}
	return StepResult{Processed: true, EventID: lastEventID, Action: "forwarded", Count: len(messages)}, nil
}

type ServerIngressRuntime struct {
	Source           MessageSource
	Consumer         rabbitmq.Consumer
	Rules            *rules.RuleSet
	Worker           apply.Worker
	EventStore       EventLogStore
	Dispatcher       DownlinkDispatcher
	EdgeNodes        []string
	NodeStore        ActiveNodeStore
	AllowCRUDCompact bool
}

type ServerIngressBatchRuntime struct {
	Source           BatchMessageSource
	Consumer         rabbitmq.Consumer
	Rules            *rules.RuleSet
	Worker           apply.Worker
	EventStore       EventLogStore
	Dispatcher       DownlinkDispatcher
	EdgeNodes        []string
	NodeStore        ActiveNodeStore
	MaxBatch         int
	FlushInterval    time.Duration
	ApplyLanes       int
	AllowCRUDCompact bool
}

type EdgeDownlinkRuntime struct {
	Source                 MessageSource
	Consumer               rabbitmq.Consumer
	Rules                  *rules.RuleSet
	Worker                 apply.Worker
	TargetDatabaseOverride string
	ConfigStore            NodeConfigStore
	AllowCRUDCompact       bool
}

type EdgeDownlinkBatchRuntime struct {
	Source                 BatchMessageSource
	Consumer               rabbitmq.Consumer
	Rules                  *rules.RuleSet
	Worker                 apply.Worker
	TargetDatabaseOverride string
	ConfigStore            NodeConfigStore
	MaxBatch               int
	FlushInterval          time.Duration
	AllowCRUDCompact       bool
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
		var raw event.SyncEvent
		if err := json.Unmarshal(cleanJSONBody(body), &raw); err != nil {
			return fmt.Errorf("parse downlink event: %w", err)
		}
		eventID = raw.EventID
		if raw.EventType == event.TypeConfigUpdate {
			if r.ConfigStore == nil {
				return fmt.Errorf("config store is required")
			}
			return r.ConfigStore.UpsertNodeConfig(ctx, nodeConfigFromEvent(raw))
		}
		evt, mapped, err := mapSyncEvent(body, r.Rules)
		if err != nil {
			return err
		}
		if err := ensureSyncModeAllowed(mapped, r.AllowCRUDCompact); err != nil {
			return err
		}
		eventID = evt.EventID
		rule := findRuleForEvent(r.Rules, evt)
		if !rule.Enable || rule.Direction == rules.DirectionIgnore {
			return nil
		}
		if !schemaChangeAllowed(evt, mapped, *rule) {
			return nil
		}
		if r.TargetDatabaseOverride != "" {
			mapped.TargetDatabase = rule.DownlinkTargetDatabase(r.TargetDatabaseOverride)
			mapped.Event.DatabaseName = mapped.TargetDatabase
		}
		if _, err := r.Worker.Apply(ctx, mapped); err != nil {
			return fmt.Errorf("apply downlink event: %w", err)
		}
		return nil
	})
	if err != nil {
		return StepResult{Processed: true, EventID: eventID, Action: "failed"}, messageFailure(err, eventID, []rabbitmq.IncomingMessage{msg}, r.Rules, r.TargetDatabaseOverride)
	}
	return StepResult{Processed: true, EventID: eventID, Action: "applied"}, nil
}

func (r EdgeDownlinkBatchRuntime) RunOnce(ctx context.Context) (StepResult, error) {
	if r.Source == nil {
		return StepResult{}, fmt.Errorf("batch message source is required")
	}
	if r.Rules == nil {
		return StepResult{}, fmt.Errorf("rules are required")
	}
	if r.Worker == nil {
		return StepResult{}, fmt.Errorf("apply worker is required")
	}
	messages, err := r.Source.GetBatch(ctx, defaultBatchSize(r.MaxBatch), defaultFlushInterval(r.FlushInterval))
	if err != nil {
		return StepResult{}, err
	}
	if len(messages) == 0 {
		return StepResult{Action: "empty"}, nil
	}

	var lastEventID string
	err = r.Consumer.HandleBatchCommit(ctx, messages, func(ctx context.Context, bodies [][]byte) (int, error) {
		count, eventID, err := r.applyDownlinkBatchBodies(ctx, bodies)
		lastEventID = eventID
		return count, err
	})
	if err != nil {
		return StepResult{Processed: true, EventID: lastEventID, Action: "failed", Count: len(messages)}, messageFailure(err, lastEventID, messages, r.Rules, r.TargetDatabaseOverride)
	}
	return StepResult{Processed: true, EventID: lastEventID, Action: "applied", Count: len(messages)}, nil
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
		if err := ensureSyncModeAllowed(mapped, r.AllowCRUDCompact); err != nil {
			return err
		}
		eventID = evt.EventID
		rule := findRuleForEvent(r.Rules, evt)
		if !rule.Enable || rule.Direction == rules.DirectionIgnore {
			return nil
		}
		if !schemaChangeAllowed(evt, mapped, *rule) {
			return nil
		}
		if r.EventStore != nil {
			// Persist first. / 先落库。 / 先に保存。
			if err := r.EventStore.UpsertEventLog(ctx, syncstore.EventLogRecord{
				Event:              evt,
				TargetDatabaseName: mapped.TargetDatabase,
				TargetTableName:    mapped.TargetTable,
				PKValue:            pkValue(evt.PrimaryKey),
				Direction:          rule.Direction,
				Status:             syncstore.StatusPending,
				Payload:            body,
			}); err != nil {
				return fmt.Errorf("persist ingress event: %w", err)
			}
		}
		if _, err := r.Worker.Apply(ctx, mapped); err != nil {
			return fmt.Errorf("apply ingress event: %w", err)
		}
		if r.EventStore != nil {
			if err := r.EventStore.UpsertEventLog(ctx, syncstore.EventLogRecord{
				Event:              evt,
				TargetDatabaseName: mapped.TargetDatabase,
				TargetTableName:    mapped.TargetTable,
				PKValue:            pkValue(evt.PrimaryKey),
				Direction:          rule.Direction,
				Status:             syncstore.StatusSuccess,
				AppliedAt:          time.Now(),
				Payload:            body,
			}); err != nil {
				return fmt.Errorf("persist applied event: %w", err)
			}
		}
		if shouldDispatch(*rule) {
			count, err := r.dispatch(ctx, evt, *rule)
			if err != nil {
				return err
			}
			dispatchCount = count
		}
		return nil
	})
	if err != nil {
		return StepResult{Processed: true, EventID: eventID, Action: "failed"}, messageFailure(err, eventID, []rabbitmq.IncomingMessage{msg}, r.Rules, "")
	}
	return StepResult{Processed: true, EventID: eventID, Action: "applied", DispatchCount: dispatchCount}, nil
}

func (r ServerIngressBatchRuntime) RunOnce(ctx context.Context) (StepResult, error) {
	if r.Source == nil {
		return StepResult{}, fmt.Errorf("batch message source is required")
	}
	if r.Rules == nil {
		return StepResult{}, fmt.Errorf("rules are required")
	}
	if r.Worker == nil {
		return StepResult{}, fmt.Errorf("apply worker is required")
	}
	messages, err := r.Source.GetBatch(ctx, defaultBatchSize(r.MaxBatch), defaultFlushInterval(r.FlushInterval))
	if err != nil {
		return StepResult{}, err
	}
	if len(messages) == 0 {
		return StepResult{Action: "empty"}, nil
	}

	var lastEventID string
	dispatchTotal := 0
	err = r.Consumer.HandleBatchCommit(ctx, messages, func(ctx context.Context, bodies [][]byte) (int, error) {
		successCount, eventID, dispatchCount, err := r.applyIngressBatchBodies(ctx, bodies)
		if eventID != "" {
			lastEventID = eventID
		}
		if err != nil {
			return successCount, err
		}
		dispatchTotal += dispatchCount
		return successCount, nil
	})
	if err != nil {
		return StepResult{Processed: true, EventID: lastEventID, Action: "failed", DispatchCount: dispatchTotal, Count: len(messages)}, messageFailure(err, lastEventID, messages, r.Rules, "")
	}
	return StepResult{Processed: true, EventID: lastEventID, Action: "applied", DispatchCount: dispatchTotal, Count: len(messages)}, nil
}

type mappedBatchEntry struct {
	body      []byte
	evt       event.SyncEvent
	mapped    mapper.MappedEvent
	rule      *rules.SyncRule
	applyable bool
}

func (r ServerIngressBatchRuntime) applyIngressBatchBodies(ctx context.Context, bodies [][]byte) (int, string, int, error) {
	entries := make([]mappedBatchEntry, 0, len(bodies))
	applyEvents := make([]mapper.MappedEvent, 0, len(bodies))
	for _, body := range bodies {
		evt, mapped, err := mapSyncEvent(body, r.Rules)
		if err != nil {
			return 0, evt.EventID, 0, err
		}
		rule := findRuleForEvent(r.Rules, evt)
		entry := mappedBatchEntry{body: body, evt: evt, mapped: mapped, rule: rule}
		if rule != nil && rule.Enable && rule.Direction != rules.DirectionIgnore && schemaChangeAllowed(evt, mapped, *rule) {
			if err := ensureSyncModeAllowed(mapped, r.AllowCRUDCompact); err != nil {
				return 0, evt.EventID, 0, err
			}
			entry.applyable = true
			applyEvents = append(applyEvents, mapped)
		}
		entries = append(entries, entry)
	}
	lastEventID := ""
	if len(entries) > 0 {
		lastEventID = entries[len(entries)-1].evt.EventID
	}

	if len(applyEvents) > 0 {
		result, err := applyBatchWithLanes(ctx, r.Worker, applyEvents, r.ApplyLanes)
		if err != nil {
			successCount := messageSuccessCountForApplyResults(entries, len(result.Results))
			if successCount > 0 && r.EventStore != nil {
				if logErr := r.persistAppliedEntries(ctx, entries[:successCount]); logErr != nil {
					return 0, lastEventID, 0, fmt.Errorf("%w; persist applied prefix failed: %v", err, logErr)
				}
			}
			if len(result.Results) < len(applyEvents) {
				lastEventID = eventIDForApplyIndex(entries, len(result.Results))
			}
			return successCount, lastEventID, 0, fmt.Errorf("apply ingress batch: %w", err)
		}
	}

	if r.EventStore != nil {
		if err := r.persistAppliedEntries(ctx, entries); err != nil {
			return 0, lastEventID, 0, fmt.Errorf("persist applied batch events: %w", err)
		}
	}

	dispatchTotal := 0
	for index, entry := range entries {
		if !entry.applyable || !shouldDispatch(*entry.rule) {
			continue
		}
		count, err := (ServerIngressRuntime{
			Dispatcher: r.Dispatcher,
			EdgeNodes:  r.EdgeNodes,
			NodeStore:  r.NodeStore,
		}).dispatch(ctx, entry.evt, *entry.rule)
		if err != nil {
			return index, entry.evt.EventID, dispatchTotal + count, err
		}
		dispatchTotal += count
	}
	return len(entries), lastEventID, dispatchTotal, nil
}

func (r ServerIngressBatchRuntime) persistAppliedEntries(ctx context.Context, entries []mappedBatchEntry) error {
	records := make([]syncstore.EventLogRecord, 0, len(entries))
	now := time.Now()
	for _, entry := range entries {
		if !entry.applyable {
			continue
		}
		records = append(records, syncstore.EventLogRecord{
			Event:              entry.evt,
			TargetDatabaseName: entry.mapped.TargetDatabase,
			TargetTableName:    entry.mapped.TargetTable,
			PKValue:            pkValue(entry.evt.PrimaryKey),
			Direction:          entry.rule.Direction,
			Status:             syncstore.StatusSuccess,
			AppliedAt:          now,
			Payload:            entry.body,
			SkipPayload:        !shouldDispatch(*entry.rule),
		})
	}
	return upsertEventLogs(ctx, r.EventStore, records)
}

func ensureSyncModeAllowed(mapped mapper.MappedEvent, allowCRUDCompact bool) error {
	if isSchemaEventType(mapped.Event.EventType) {
		return nil
	}
	if mapped.SyncMode == rules.SyncModeCRUDCompact && !allowCRUDCompact {
		return fmt.Errorf("sync_mode %s requires sync.enable_crud_compact", rules.SyncModeCRUDCompact)
	}
	return nil
}

func schemaChangeAllowed(evt event.SyncEvent, mapped mapper.MappedEvent, rule rules.SyncRule) bool {
	if !isSchemaEventType(evt.EventType) {
		return true
	}
	if evt.SchemaChange == nil || !mapped.SchemaChangeSelected {
		return false
	}
	switch evt.EventType {
	case event.TypeAddColumn:
		return rule.SchemaSync.AddColumns
	case event.TypeDropColumn:
		return rule.SchemaSync.DropColumns && (rule.Direction == rules.DirectionServerToEdge || len(rule.SourceNodeIDs) == 1)
	default:
		return false
	}
}

func isSchemaEventType(eventType string) bool {
	return eventType == event.TypeAddColumn || eventType == event.TypeDropColumn
}

func applyBatch(ctx context.Context, worker apply.Worker, events []mapper.MappedEvent) (apply.BatchResult, error) {
	defer measurePhase(ctx, "mysql_apply")()
	if batchWorker, ok := worker.(apply.BatchWorker); ok {
		return batchWorker.ApplyBatch(ctx, events)
	}
	results := make([]apply.Result, 0, len(events))
	for _, evt := range events {
		result, err := worker.Apply(ctx, evt)
		if err != nil {
			return apply.BatchResult{Results: results}, err
		}
		results = append(results, result)
	}
	return apply.BatchResult{Results: results}, nil
}

type laneItem struct {
	index int
	event mapper.MappedEvent
}

type laneResult struct {
	results []apply.Result
	indices []int
	err     error
}

func applyBatchWithLanes(ctx context.Context, worker apply.Worker, events []mapper.MappedEvent, lanes int) (apply.BatchResult, error) {
	if lanes <= 1 || len(events) <= 1 || containsCompactMode(events) {
		return applyBatch(ctx, worker, events)
	}
	laneCount := lanes
	if laneCount > len(events) {
		laneCount = len(events)
	}
	laneEvents := make([][]laneItem, laneCount)
	for index, evt := range events {
		lane := int(stableHash(applyLaneKey(evt)) % uint32(laneCount))
		laneEvents[lane] = append(laneEvents[lane], laneItem{index: index, event: evt})
	}

	out := make(chan laneResult, laneCount)
	var wg sync.WaitGroup
	for _, items := range laneEvents {
		if len(items) == 0 {
			continue
		}
		wg.Add(1)
		go func(items []laneItem) {
			defer wg.Done()
			batch := make([]mapper.MappedEvent, 0, len(items))
			indices := make([]int, 0, len(items))
			for _, item := range items {
				batch = append(batch, item.event)
				indices = append(indices, item.index)
			}
			result, err := applyBatch(ctx, worker, batch)
			out <- laneResult{results: result.Results, indices: indices, err: err}
		}(items)
	}
	wg.Wait()
	close(out)

	committed := make([]bool, len(events))
	byIndex := make([]apply.Result, len(events))
	var firstErr error
	firstFailedIndex := len(events)
	for result := range out {
		for i, applied := range result.results {
			if i >= len(result.indices) {
				break
			}
			index := result.indices[i]
			committed[index] = true
			byIndex[index] = applied
		}
		if result.err != nil {
			failedIndex := len(events)
			if len(result.results) < len(result.indices) {
				failedIndex = result.indices[len(result.results)]
			}
			if failedIndex < firstFailedIndex {
				firstFailedIndex = failedIndex
				firstErr = result.err
			}
		}
	}

	results := make([]apply.Result, 0, len(events))
	for index := 0; index < len(events); index++ {
		if !committed[index] {
			break
		}
		results = append(results, byIndex[index])
	}
	if firstErr != nil {
		return apply.BatchResult{Results: results}, firstErr
	}
	if len(results) != len(events) {
		return apply.BatchResult{Results: results}, fmt.Errorf("parallel apply committed non-prefix events only")
	}
	return apply.BatchResult{Results: results}, nil
}

func containsCompactMode(events []mapper.MappedEvent) bool {
	for _, evt := range events {
		if evt.SyncMode == rules.SyncModeCRUDCompact {
			return true
		}
	}
	return false
}

func applyLaneKey(evt mapper.MappedEvent) string {
	if evt.SyncMode == rules.SyncModeAppendOnly {
		return "append|" + evt.TargetDatabase + "." + evt.TargetTable
	}
	return "crud|" + evt.TargetDatabase + "." + evt.TargetTable + "|" + pkValue(evt.TargetPrimaryKey)
}

func stableHash(value string) uint32 {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(value))
	return hash.Sum32()
}

func upsertEventLogs(ctx context.Context, store EventLogStore, records []syncstore.EventLogRecord) error {
	defer measurePhase(ctx, "event_log")()
	if len(records) == 0 {
		return nil
	}
	if batchStore, ok := store.(EventLogBatchStore); ok {
		return batchStore.UpsertEventLogs(ctx, records)
	}
	for _, record := range records {
		if err := store.UpsertEventLog(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func completeEventLogs(ctx context.Context, store EventLogStore, records []syncstore.EventLogRecord) error {
	if len(records) == 0 {
		return nil
	}
	at := time.Now()
	if successStore, ok := store.(EventLogSuccessStore); ok {
		defer measurePhase(ctx, "event_log")()
		ids := make([]string, len(records))
		for i := range records {
			ids[i] = records[i].Event.EventID
		}
		return successStore.MarkEventLogsSucceeded(ctx, ids, at)
	}
	for i := range records {
		records[i].Status, records[i].AppliedAt = syncstore.StatusSuccess, at
	}
	return upsertEventLogs(ctx, store, records)
}

func messageSuccessCountForApplyResults(entries []mappedBatchEntry, appliedCount int) int {
	seen := 0
	for index, entry := range entries {
		if !entry.applyable {
			continue
		}
		if seen == appliedCount {
			return index
		}
		seen++
	}
	return len(entries)
}

func eventIDForApplyIndex(entries []mappedBatchEntry, applyIndex int) string {
	seen := 0
	for _, entry := range entries {
		if !entry.applyable {
			continue
		}
		if seen == applyIndex {
			return entry.evt.EventID
		}
		seen++
	}
	if len(entries) == 0 {
		return ""
	}
	return entries[len(entries)-1].evt.EventID
}

func (r ServerIngressRuntime) dispatch(ctx context.Context, evt event.SyncEvent, rule rules.SyncRule) (int, error) {
	if r.Dispatcher == nil {
		return 0, nil
	}
	nodeIDs, err := dispatchNodeIDs(ctx, rule, r.EdgeNodes, r.NodeStore)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, nodeID := range nodeIDs {
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

func dispatchNodeIDs(ctx context.Context, rule rules.SyncRule, edgeNodes []string, nodeStore ActiveNodeStore) ([]string, error) {
	nodeIDs := rule.DispatchNodeIDs
	if dispatchTarget(rule) == rules.DispatchActiveEdges {
		nodeIDs = edgeNodes
	}
	if len(nodeIDs) == 0 && dispatchTarget(rule) == rules.DispatchActiveEdges && nodeStore != nil {
		var err error
		nodeIDs, err = nodeStore.ListActiveEdgeNodeIDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("list active edge nodes: %w", err)
		}
	}
	return nodeIDs, nil
}

type ReplayRuntime struct {
	Store      ReplayStore
	Dispatcher DownlinkDispatcher
	Limit      int
}

func (r ReplayRuntime) RunOnce(ctx context.Context) (StepResult, error) {
	if r.Store == nil {
		return StepResult{}, fmt.Errorf("replay store is required")
	}
	if r.Dispatcher == nil {
		return StepResult{}, fmt.Errorf("downlink dispatcher is required")
	}
	limit := r.Limit
	if limit <= 0 {
		limit = 1
	}
	items, err := r.Store.ListPendingReplays(ctx, limit)
	if err != nil {
		return StepResult{}, err
	}
	if len(items) == 0 {
		return StepResult{Action: "empty"}, nil
	}

	item := items[0]
	var evt event.SyncEvent
	if err := json.Unmarshal(item.Payload, &evt); err != nil {
		_ = r.Store.UpsertAck(ctx, syncstore.AckRecord{
			EventID:      item.EventID,
			TargetNodeID: item.TargetNodeID,
			Status:       syncstore.StatusFailed,
			ErrorMessage: "invalid replay payload",
		})
		return StepResult{Processed: true, EventID: item.EventID, Action: "failed"}, fmt.Errorf("parse replay event: %w", err)
	}
	// Replay only pending. / 只重放待处理。 / 保留だけ再送。
	if err := r.Dispatcher.Dispatch(ctx, evt, item.TargetNodeID); err != nil {
		_ = r.Store.UpsertAck(ctx, syncstore.AckRecord{
			EventID:      item.EventID,
			TargetNodeID: item.TargetNodeID,
			Status:       syncstore.StatusFailed,
			ErrorMessage: err.Error(),
		})
		return StepResult{Processed: true, EventID: item.EventID, Action: "failed"}, fmt.Errorf("replay dispatch event %s to %s: %w", item.EventID, item.TargetNodeID, err)
	}
	if err := r.Store.UpsertDispatch(ctx, syncstore.DispatchRecord{
		EventID:      item.EventID,
		TargetNodeID: item.TargetNodeID,
		Status:       syncstore.StatusSuccess,
	}); err != nil {
		return StepResult{Processed: true, EventID: item.EventID, Action: "failed"}, err
	}
	if err := r.Store.UpsertAck(ctx, syncstore.AckRecord{
		EventID:      item.EventID,
		TargetNodeID: item.TargetNodeID,
		Status:       syncstore.StatusSuccess,
	}); err != nil {
		return StepResult{Processed: true, EventID: item.EventID, Action: "failed"}, err
	}
	return StepResult{Processed: true, EventID: item.EventID, Action: "replayed", DispatchCount: 1}, nil
}

func shouldDispatch(rule rules.SyncRule) bool {
	switch dispatchTarget(rule) {
	case rules.DispatchNone:
		return false
	case rules.DispatchActiveEdges, rules.DispatchSelectedEdges:
		return true
	default:
		return false
	}
}

func dispatchTarget(rule rules.SyncRule) string {
	switch rule.DispatchTarget {
	case "", rules.DispatchAuto:
		if rule.Direction == rules.DirectionBidirectional || rule.Direction == rules.DirectionServerToEdge {
			return rules.DispatchActiveEdges
		}
		return rules.DispatchNone
	default:
		return rule.DispatchTarget
	}
}

func mapSyncEvent(body []byte, ruleSet *rules.RuleSet) (event.SyncEvent, mapper.MappedEvent, error) {
	var evt event.SyncEvent
	if err := json.Unmarshal(cleanJSONBody(body), &evt); err != nil {
		return event.SyncEvent{}, mapper.MappedEvent{}, fmt.Errorf("parse sync event: %w", err)
	}
	rule := findRuleForEvent(ruleSet, evt)
	if rule == nil {
		return evt, mapper.MappedEvent{}, describeEventFailure(fmt.Errorf("sync rule not found for %s.%s", evt.DatabaseName, evt.TableName), evt, nil, "")
	}
	if !rule.Enable || rule.Direction == rules.DirectionIgnore {
		return evt, mapper.MappedEvent{}, describeEventFailure(fmt.Errorf("rule_not_active: queued event requires an enabled non-IGNORE rule"), evt, rule, "")
	}
	mapped, err := mapper.MapEvent(evt, *rule)
	if err != nil {
		return evt, mapper.MappedEvent{}, describeEventFailure(fmt.Errorf("map sync event: %w", err), evt, rule, "")
	}
	return evt, mapped, nil
}

func findRuleForEvent(ruleSet *rules.RuleSet, evt event.SyncEvent) *rules.SyncRule {
	if ruleSet == nil {
		return nil
	}
	return ruleSet.FindForNode(evt.DatabaseName, evt.TableName, evt.OriginNodeID, evt.SourceNodeID)
}

func pkValue(primaryKey map[string]any) string {
	if len(primaryKey) == 0 {
		return ""
	}
	keys := make([]string, 0, len(primaryKey))
	for key := range primaryKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, primaryKey[key]))
	}
	return strings.Join(parts, ",")
}

func nodeConfigFromEvent(evt event.SyncEvent) syncstore.NodeConfig {
	cfg := syncstore.NodeConfig{NodeID: evt.TargetNodeID}
	if cfg.NodeID == "" {
		cfg.NodeID = stringMapValue(evt.PrimaryKey, "node_id")
	}
	cfg.MySQLHost = stringMapValue(evt.After, "mysql_host")
	cfg.MySQLPort = intMapValue(evt.After, "mysql_port")
	cfg.MySQLDatabase = stringMapValue(evt.After, "mysql_database")
	cfg.MySQLUsername = stringMapValue(evt.After, "mysql_username")
	cfg.CDCType = stringMapValue(evt.After, "cdc_type")
	cfg.CDCFilter = stringMapValue(evt.After, "cdc_filter")
	cfg.CDCBatchSize = intMapValue(evt.After, "cdc_batch_size")
	cfg.CDCDestination = stringMapValue(evt.After, "cdc_destination")
	cfg.RuleVersion = int64MapValue(evt.After, "rule_version")
	return cfg
}

func stringMapValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	if value, ok := values[key].(string); ok {
		return value
	}
	return ""
}

func intMapValue(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func int64MapValue(values map[string]any, key string) int64 {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	default:
		return 0
	}
}

func eventIDFromBody(body []byte) (string, error) {
	var evt event.SyncEvent
	if err := json.Unmarshal(cleanJSONBody(body), &evt); err != nil {
		return "", fmt.Errorf("parse upload event: %w", err)
	}
	return evt.EventID, nil
}

func cleanJSONBody(body []byte) []byte {
	return bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
}

func defaultBatchSize(value int) int {
	if value > 0 {
		return value
	}
	return DefaultBatchSize
}

func defaultFlushInterval(value time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return DefaultFlushInterval
}
