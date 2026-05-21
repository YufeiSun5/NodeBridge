package normalizer

import (
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
)

func TestNormalizeChangeEvent(t *testing.T) {
	now := time.Date(2026, 5, 21, 12, 30, 0, 0, time.UTC)
	eventTime := time.Date(2026, 5, 21, 12, 29, 0, 0, time.UTC)
	n := New(Options{
		NodeID:        "edge-001",
		SchemaVersion: 7,
		Now:           func() time.Time { return now },
		NewEventID:    fixedID("evt-001"),
	})

	evt, err := n.Normalize(cdc.ChangeEvent{
		DatabaseName: "scada_edge",
		TableName:    "device_config",
		Operation:    cdc.OperationUpdate,
		PrimaryKey:   map[string]any{"id": 1},
		Before:       map[string]any{"id": 1, "name": "Old"},
		After:        map[string]any{"id": 1, "name": "New", "sync_version": 3},
		BinlogFile:   "mysql-bin.000001",
		BinlogPos:    42,
		EventTime:    eventTime,
	})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	if evt.EventID != "evt-001" || evt.TraceID != "evt-001" {
		t.Fatalf("unexpected ids %+v", evt)
	}
	if evt.EventType != string(cdc.OperationUpdate) || evt.OriginNodeID != "edge-001" || evt.SourceNodeID != "edge-001" {
		t.Fatalf("unexpected event identity %+v", evt)
	}
	if evt.SchemaVersion != 7 || evt.SyncVersion != 3 {
		t.Fatalf("unexpected versions %+v", evt)
	}
	if evt.BinlogFile != "mysql-bin.000001" || evt.BinlogPos != 42 {
		t.Fatalf("unexpected binlog fields %+v", evt)
	}
	if !evt.CreatedAt.Equal(now) || !evt.EventTime.Equal(eventTime) {
		t.Fatalf("unexpected times created=%s event=%s", evt.CreatedAt, evt.EventTime)
	}
}

func TestNormalizeUsesNowWhenEventTimeMissing(t *testing.T) {
	now := time.Date(2026, 5, 21, 12, 30, 0, 0, time.UTC)
	evt, err := New(Options{
		NodeID:     "edge-001",
		Now:        func() time.Time { return now },
		NewEventID: fixedID("evt-001"),
	}).Normalize(cdc.ChangeEvent{
		DatabaseName: "scada_edge",
		TableName:    "alarm_history",
		Operation:    cdc.OperationInsert,
		PrimaryKey:   map[string]any{"id": 1},
		After:        map[string]any{"id": 1},
	})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if !evt.EventTime.Equal(now) {
		t.Fatalf("expected event time fallback, got %s", evt.EventTime)
	}
}

func TestNormalizeRejectsInvalidInput(t *testing.T) {
	n := New(Options{NewEventID: fixedID("evt-001")})
	if _, err := n.Normalize(cdc.ChangeEvent{DatabaseName: "db", TableName: "t", Operation: cdc.OperationInsert}); err == nil {
		t.Fatal("expected missing node id error")
	}

	n = New(Options{NodeID: "edge-001", NewEventID: fixedID("evt-001")})
	if _, err := n.Normalize(cdc.ChangeEvent{DatabaseName: "db", TableName: "t", Operation: "TRUNCATE"}); err == nil {
		t.Fatal("expected unsupported operation error")
	}
	if _, err := n.Normalize(cdc.ChangeEvent{Operation: cdc.OperationInsert}); err == nil {
		t.Fatal("expected missing table identity error")
	}
}

func TestRandomEventID(t *testing.T) {
	now := time.Date(2026, 5, 21, 12, 30, 0, 0, time.UTC)
	id, err := RandomEventID(now)
	if err != nil {
		t.Fatalf("RandomEventID returned error: %v", err)
	}
	if len(id) < 20 {
		t.Fatalf("event id too short: %q", id)
	}
}

func fixedID(value string) IDGenerator {
	return func(now time.Time) (string, error) {
		return value, nil
	}
}
