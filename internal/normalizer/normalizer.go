package normalizer

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/dbgovernance"
	"github.com/YufeiSun5/NodeBridge/internal/event"
)

type IDGenerator func(now time.Time) (string, error)
type ChangeIDGenerator func(change cdc.ChangeEvent, nodeID string, now time.Time) (string, error)

type Options struct {
	NodeID        string
	SchemaVersion int64
	Now           func() time.Time
	NewEventID    IDGenerator
	NewChangeID   ChangeIDGenerator
}

type Normalizer struct {
	options Options
}

func New(options Options) Normalizer {
	if options.Now == nil {
		options.Now = time.Now
	}
	hasCustomEventID := options.NewEventID != nil
	if options.NewEventID == nil {
		options.NewEventID = RandomEventID
	}
	if options.NewChangeID == nil {
		if hasCustomEventID {
			options.NewChangeID = func(change cdc.ChangeEvent, nodeID string, now time.Time) (string, error) {
				return "", nil
			}
		} else {
			options.NewChangeID = StableCDCEventID
		}
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
	timeSource := "source_event"
	if change.BinlogFile != "" && change.BinlogPos > 0 {
		timeSource = "source_binlog"
	}
	if eventTime.IsZero() {
		eventTime = now
		timeSource = "processing_fallback"
	}
	eventID, err := n.options.NewChangeID(change, n.options.NodeID, now)
	if err != nil {
		return event.SyncEvent{}, err
	}
	if eventID == "" {
		eventID, err = n.options.NewEventID(now)
		if err != nil {
			return event.SyncEvent{}, err
		}
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
		SchemaChange:  cloneSchemaChange(change.SchemaChange),
		BinlogFile:    change.BinlogFile,
		BinlogPos:     change.BinlogPos,
		SchemaVersion: n.options.SchemaVersion,
		SyncVersion:   int64Value(change.After, "sync_version"),
		CreatedAt:     now,
		EventTime:     eventTime,
		TraceID:       eventID,
		Headers: map[string]string{
			"normalizer":        "cdc",
			"event_time_source": timeSource,
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

func StableCDCEventID(change cdc.ChangeEvent, nodeID string, now time.Time) (string, error) {
	_ = now
	if change.BinlogFile == "" || change.BinlogPos == 0 || (len(change.PrimaryKey) == 0 && change.SchemaChange == nil) {
		return "", nil
	}
	key := struct {
		NodeID       string         `json:"node_id"`
		Database     string         `json:"database"`
		Table        string         `json:"table"`
		Operation    cdc.Operation  `json:"operation"`
		BinlogFile   string         `json:"binlog_file"`
		BinlogPos    uint32         `json:"binlog_pos"`
		PrimaryKey   map[string]any `json:"primary_key"`
		Before       map[string]any `json:"before,omitempty"`
		After        map[string]any `json:"after,omitempty"`
		SchemaChange any            `json:"schema_change,omitempty"`
	}{
		NodeID:       nodeID,
		Database:     change.DatabaseName,
		Table:        change.TableName,
		Operation:    change.Operation,
		BinlogFile:   change.BinlogFile,
		BinlogPos:    change.BinlogPos,
		PrimaryKey:   change.PrimaryKey,
		Before:       change.Before,
		After:        change.After,
		SchemaChange: change.SchemaChange,
	}
	body, err := json.Marshal(key)
	if err != nil {
		return "", fmt.Errorf("build stable event id: %w", err)
	}
	sum := sha256.Sum256(body)
	return "cdc" + hex.EncodeToString(sum[:])[:29], nil
}

func validOperation(operation cdc.Operation) bool {
	switch operation {
	case cdc.OperationInsert, cdc.OperationUpdate, cdc.OperationDelete, cdc.OperationAddColumn, cdc.OperationDropColumn:
		return true
	default:
		return false
	}
}

func cloneSchemaChange(change *dbgovernance.SchemaChange) *dbgovernance.SchemaChange {
	if change == nil {
		return nil
	}
	cloned := *change
	return &cloned
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
	case string:
		parsed, _ := strconv.ParseInt(value, 10, 64)
		return parsed
	case json.Number:
		parsed, _ := value.Int64()
		return parsed
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
