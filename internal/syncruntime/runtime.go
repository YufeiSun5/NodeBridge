package syncruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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

type DownlinkDispatcher interface {
	Dispatch(ctx context.Context, evt event.SyncEvent, targetNodeID string) error
}

type EventLogStore interface {
	UpsertEventLog(ctx context.Context, record syncstore.EventLogRecord) error
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
	EventStore EventLogStore
	Dispatcher DownlinkDispatcher
	EdgeNodes  []string
	NodeStore  ActiveNodeStore
}

type EdgeDownlinkRuntime struct {
	Source                 MessageSource
	Consumer               rabbitmq.Consumer
	Rules                  *rules.RuleSet
	Worker                 apply.Worker
	TargetDatabaseOverride string
	ConfigStore            NodeConfigStore
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
		if err := json.Unmarshal(body, &raw); err != nil {
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
		eventID = evt.EventID
		rule := r.Rules.Find(evt.DatabaseName, evt.TableName)
		if !rule.Enable || rule.Direction == rules.DirectionIgnore {
			return nil
		}
		if r.TargetDatabaseOverride != "" {
			// Local DB wins. / 本地库优先。 / ローカルDB優先。
			mapped.TargetDatabase = r.TargetDatabaseOverride
			mapped.Event.DatabaseName = r.TargetDatabaseOverride
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
	nodeIDs := r.EdgeNodes
	if len(nodeIDs) == 0 && r.NodeStore != nil {
		var err error
		nodeIDs, err = r.NodeStore.ListActiveEdgeNodeIDs(ctx)
		if err != nil {
			return 0, fmt.Errorf("list active edge nodes: %w", err)
		}
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
