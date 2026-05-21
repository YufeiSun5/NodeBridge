package normalizer

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/event"
)

type IDGenerator func(now time.Time) (string, error)

type Options struct {
	NodeID        string
	SchemaVersion int64
	Now           func() time.Time
	NewEventID    IDGenerator
}

type Normalizer struct {
	options Options
}

func New(options Options) Normalizer {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.NewEventID == nil {
		options.NewEventID = RandomEventID
	}
	return Normalizer{options: options}
}

func (n Normalizer) Normalize(change cdc.ChangeEvent) (event.SyncEvent, error) {
	if n.options.NodeID == "" {
		return event.SyncEvent{}, fmt.Errorf("node id is required")
	}
	if change.DatabaseName == "" || change.TableName == "" {
		return event.SyncEvent{}, fmt.Errorf("database and table are required")
	}
	if !validOperation(change.Operation) {
		return event.SyncEvent{}, fmt.Errorf("unsupported cdc operation %q", change.Operation)
	}

	now := n.options.Now()
	eventTime := change.EventTime
	if eventTime.IsZero() {
		eventTime = now
	}
	eventID, err := n.options.NewEventID(now)
	if err != nil {
		return event.SyncEvent{}, err
	}

	return event.SyncEvent{
		EventID:       eventID,
		EventType:     string(change.Operation),
		OriginNodeID:  n.options.NodeID,
		SourceNodeID:  n.options.NodeID,
		DatabaseName:  change.DatabaseName,
		TableName:     change.TableName,
		PrimaryKey:    cloneMap(change.PrimaryKey),
		Before:        cloneMap(change.Before),
		After:         cloneMap(change.After),
		BinlogFile:    change.BinlogFile,
		BinlogPos:     change.BinlogPos,
		SchemaVersion: n.options.SchemaVersion,
		SyncVersion:   int64Value(change.After, "sync_version"),
		CreatedAt:     now,
		EventTime:     eventTime,
		TraceID:       eventID,
		Headers: map[string]string{
			"normalizer": "cdc",
		},
	}, nil
}

func RandomEventID(now time.Time) (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate event id: %w", err)
	}
	return fmt.Sprintf("%016x%s", now.UnixNano(), hex.EncodeToString(random)), nil
}

func validOperation(operation cdc.Operation) bool {
	switch operation {
	case cdc.OperationInsert, cdc.OperationUpdate, cdc.OperationDelete:
		return true
	default:
		return false
	}
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func int64Value(values map[string]any, key string) int64 {
	switch value := values[key].(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case int32:
		return int64(value)
	case float64:
		return int64(value)
	default:
		return 0
	}
}
