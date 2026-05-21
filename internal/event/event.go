package event

import "time"

const (
	TypeInsert       = "INSERT"
	TypeUpdate       = "UPDATE"
	TypeDelete       = "DELETE"
	TypeHeartbeat    = "HEARTBEAT"
	TypeConfigUpdate = "CONFIG_UPDATE"
)

type SyncEvent struct {
	EventID       string            `json:"event_id"`
	EventType     string            `json:"event_type"`
	OriginNodeID  string            `json:"origin_node_id"`
	SourceNodeID  string            `json:"source_node_id"`
	TargetNodeID  string            `json:"target_node_id,omitempty"`
	DatabaseName  string            `json:"database_name"`
	TableName     string            `json:"table_name"`
	PrimaryKey    map[string]any    `json:"primary_key"`
	Before        map[string]any    `json:"before,omitempty"`
	After         map[string]any    `json:"after,omitempty"`
	BinlogFile    string            `json:"binlog_file,omitempty"`
	BinlogPos     uint32            `json:"binlog_pos,omitempty"`
	GTID          string            `json:"gtid,omitempty"`
	SchemaVersion int64             `json:"schema_version"`
	SyncVersion   int64             `json:"sync_version,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	EventTime     time.Time         `json:"event_time"`
	TraceID       string            `json:"trace_id"`
	Headers       map[string]string `json:"headers,omitempty"`
}
