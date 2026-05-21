package syncstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/event"
)

const (
	StatusPending = "PENDING"
	StatusSuccess = "SUCCESS"
	StatusFailed  = "FAILED"
)

type Store struct {
	DB    *sql.DB
	Clock func() time.Time
}

func New(db *sql.DB) *Store {
	return &Store{DB: db, Clock: time.Now}
}

type AckRecord struct {
	EventID      string
	TargetNodeID string
	Status       string
	AckAt        time.Time
	ErrorMessage string
}

type DispatchRecord struct {
	EventID      string
	TargetNodeID string
	Status       string
	DispatchedAt time.Time
}

type ErrorRecord struct {
	EventID      string
	Module       string
	ErrorMessage string
	CreatedAt    time.Time
}

type FailedEvent struct {
	EventID      string
	TargetNodeID string
	Status       string
	ErrorMessage string
	CreatedAt    time.Time
}

type EventLogRecord struct {
	Event              event.SyncEvent
	TargetDatabaseName string
	TargetTableName    string
	PKValue            string
	Direction          string
	Status             string
	ReceivedAt         time.Time
	AppliedAt          time.Time
	ErrorMessage       string
	Payload            []byte
}

type ReplayEvent struct {
	EventID      string
	TargetNodeID string
	Payload      []byte
	CreatedAt    time.Time
}

func (s *Store) UpsertEventLog(ctx context.Context, record EventLogRecord) error {
	if s.DB == nil {
		return fmt.Errorf("sync store db is required")
	}
	if record.Event.EventID == "" || record.Event.DatabaseName == "" || record.Event.TableName == "" {
		return fmt.Errorf("event_id, database_name and table_name are required")
	}
	if record.Status == "" {
		return fmt.Errorf("event status is required")
	}
	payload := record.Payload
	if len(payload) == 0 {
		encoded, err := json.Marshal(record.Event)
		if err != nil {
			return fmt.Errorf("encode event payload: %w", err)
		}
		payload = encoded
	}
	receivedAt := record.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = s.now()
	}
	eventTime := record.Event.EventTime
	if eventTime.IsZero() {
		eventTime = receivedAt
	}

	_, err := s.DB.ExecContext(ctx, `
INSERT INTO sync_event_log (
  event_id, origin_node_id, source_node_id, database_name, table_name,
  target_database_name, target_table_name, pk_value, op_type, direction,
  status, event_time, received_at, applied_at, error_message, event_payload
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  target_database_name = VALUES(target_database_name),
  target_table_name = VALUES(target_table_name),
  status = VALUES(status),
  applied_at = VALUES(applied_at),
  error_message = VALUES(error_message),
  event_payload = VALUES(event_payload)
`, record.Event.EventID,
		record.Event.OriginNodeID,
		record.Event.SourceNodeID,
		record.Event.DatabaseName,
		record.Event.TableName,
		nullableString(record.TargetDatabaseName),
		nullableString(record.TargetTableName),
		record.PKValue,
		record.Event.EventType,
		record.Direction,
		record.Status,
		eventTime,
		receivedAt,
		nullableTime(record.AppliedAt),
		nullableString(record.ErrorMessage),
		string(payload),
	)
	if err != nil {
		return fmt.Errorf("upsert event log: %w", err)
	}
	return nil
}

func (s *Store) UpsertAck(ctx context.Context, record AckRecord) error {
	if s.DB == nil {
		return fmt.Errorf("sync store db is required")
	}
	if record.EventID == "" || record.TargetNodeID == "" || record.Status == "" {
		return fmt.Errorf("ack event_id, target_node_id and status are required")
	}
	now := s.now()
	ackAt := nullableTime(record.AckAt)
	if record.Status == StatusSuccess && record.AckAt.IsZero() {
		ackAt = now
	}

	_, err := s.DB.ExecContext(ctx, `
INSERT INTO sync_ack_log (event_id, target_node_id, status, ack_at, error_message, created_at)
VALUES (?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  ack_at = VALUES(ack_at),
  error_message = VALUES(error_message)
`, record.EventID, record.TargetNodeID, record.Status, ackAt, nullableString(record.ErrorMessage), now)
	if err != nil {
		return fmt.Errorf("upsert ack log: %w", err)
	}
	return nil
}

func (s *Store) UpsertDispatch(ctx context.Context, record DispatchRecord) error {
	if s.DB == nil {
		return fmt.Errorf("sync store db is required")
	}
	if record.EventID == "" || record.TargetNodeID == "" || record.Status == "" {
		return fmt.Errorf("dispatch event_id, target_node_id and status are required")
	}
	now := s.now()
	dispatchedAt := nullableTime(record.DispatchedAt)
	if record.Status == StatusSuccess && record.DispatchedAt.IsZero() {
		dispatchedAt = now
	}

	_, err := s.DB.ExecContext(ctx, `
INSERT INTO sync_dispatch_log (event_id, target_node_id, status, dispatched_at, created_at)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  dispatched_at = VALUES(dispatched_at)
`, record.EventID, record.TargetNodeID, record.Status, dispatchedAt, now)
	if err != nil {
		return fmt.Errorf("upsert dispatch log: %w", err)
	}
	return nil
}

func (s *Store) InsertError(ctx context.Context, record ErrorRecord) error {
	if s.DB == nil {
		return fmt.Errorf("sync store db is required")
	}
	if record.Module == "" || record.ErrorMessage == "" {
		return fmt.Errorf("error module and message are required")
	}
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.now()
	}

	_, err := s.DB.ExecContext(ctx, `
INSERT INTO sync_error_log (event_id, module, error_message, created_at)
VALUES (?, ?, ?, ?)
`, nullableString(record.EventID), record.Module, record.ErrorMessage, createdAt)
	if err != nil {
		return fmt.Errorf("insert error log: %w", err)
	}
	return nil
}

func (s *Store) ListFailedEvents(ctx context.Context, limit int) ([]FailedEvent, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("sync store db is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	rows, err := s.DB.QueryContext(ctx, `
SELECT event_id, target_node_id, status, COALESCE(error_message, ''), created_at
FROM sync_ack_log
WHERE status = ?
ORDER BY created_at DESC
LIMIT ?
`, StatusFailed, limit)
	if err != nil {
		return nil, fmt.Errorf("list failed events: %w", err)
	}
	defer rows.Close()

	var result []FailedEvent
	for rows.Next() {
		var event FailedEvent
		if err := rows.Scan(&event.EventID, &event.TargetNodeID, &event.Status, &event.ErrorMessage, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan failed event: %w", err)
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate failed events: %w", err)
	}
	return result, nil
}

func (s *Store) ListPendingReplays(ctx context.Context, limit int) ([]ReplayEvent, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("sync store db is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	rows, err := s.DB.QueryContext(ctx, `
SELECT a.event_id, a.target_node_id, e.event_payload, a.created_at
FROM sync_ack_log a
JOIN sync_event_log e ON e.event_id = a.event_id
WHERE a.status = ? AND e.event_payload IS NOT NULL
ORDER BY a.created_at ASC
LIMIT ?
`, StatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending replays: %w", err)
	}
	defer rows.Close()

	var result []ReplayEvent
	for rows.Next() {
		var item ReplayEvent
		var payload string
		if err := rows.Scan(&item.EventID, &item.TargetNodeID, &payload, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending replay: %w", err)
		}
		item.Payload = []byte(payload)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending replays: %w", err)
	}
	return result, nil
}

func (s *Store) MarkRetryPending(ctx context.Context, eventID, targetNodeID string) error {
	return s.UpsertAck(ctx, AckRecord{
		EventID:      eventID,
		TargetNodeID: targetNodeID,
		Status:       StatusPending,
	})
}

func (s *Store) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
