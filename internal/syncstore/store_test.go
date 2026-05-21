package syncstore

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/event"
)

func TestStoreUpsertAckSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := New(db)
	store.Clock = fixedTime

	mock.ExpectExec("INSERT INTO sync_ack_log").
		WithArgs("evt-001", "edge-002", StatusSuccess, fixedTime(), nil, fixedTime()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.UpsertAck(context.Background(), AckRecord{
		EventID:      "evt-001",
		TargetNodeID: "edge-002",
		Status:       StatusSuccess,
	})
	if err != nil {
		t.Fatalf("UpsertAck returned error: %v", err)
	}
	assertExpectations(t, mock)
}

func TestStoreUpsertDispatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := New(db)
	store.Clock = fixedTime

	mock.ExpectExec("INSERT INTO sync_dispatch_log").
		WithArgs("evt-001", "edge-002", StatusSuccess, fixedTime(), fixedTime()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.UpsertDispatch(context.Background(), DispatchRecord{
		EventID:      "evt-001",
		TargetNodeID: "edge-002",
		Status:       StatusSuccess,
	})
	if err != nil {
		t.Fatalf("UpsertDispatch returned error: %v", err)
	}
	assertExpectations(t, mock)
}

func TestStoreInsertError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := New(db)
	store.Clock = fixedTime

	mock.ExpectExec("INSERT INTO sync_error_log").
		WithArgs("evt-001", "server-ingress", "apply failed", fixedTime()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.InsertError(context.Background(), ErrorRecord{
		EventID:      "evt-001",
		Module:       "server-ingress",
		ErrorMessage: "apply failed",
	})
	if err != nil {
		t.Fatalf("InsertError returned error: %v", err)
	}
	assertExpectations(t, mock)
}

func TestStoreUpsertEventLog(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := New(db)
	store.Clock = fixedTime
	evt := sampleSyncEvent()

	mock.ExpectExec("INSERT INTO sync_event_log").
		WithArgs(
			"evt-001",
			"edge-001",
			"edge-001",
			"scada_edge",
			"device_config",
			"scada_center",
			"device_settings",
			"id=1",
			"UPDATE",
			"BIDIRECTIONAL",
			StatusSuccess,
			evt.EventTime,
			fixedTime(),
			nil,
			nil,
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.UpsertEventLog(context.Background(), EventLogRecord{
		Event:              evt,
		TargetDatabaseName: "scada_center",
		TargetTableName:    "device_settings",
		PKValue:            "id=1",
		Direction:          "BIDIRECTIONAL",
		Status:             StatusSuccess,
	})
	if err != nil {
		t.Fatalf("UpsertEventLog returned error: %v", err)
	}
	assertExpectations(t, mock)
}

func TestStoreListFailedEvents(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT event_id, target_node_id, status, COALESCE(error_message, ''), created_at
FROM sync_ack_log
WHERE status = ?
ORDER BY created_at DESC
LIMIT ?
`)).
		WithArgs(StatusFailed, 50).
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "target_node_id", "status", "error_message", "created_at"}).
			AddRow("evt-001", "edge-002", StatusFailed, "apply failed", fixedTime()))

	events, err := New(db).ListFailedEvents(context.Background(), 50)
	if err != nil {
		t.Fatalf("ListFailedEvents returned error: %v", err)
	}
	if len(events) != 1 || events[0].EventID != "evt-001" {
		t.Fatalf("unexpected failed events %+v", events)
	}
	assertExpectations(t, mock)
}

func TestStoreListPendingReplays(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT a.event_id, a.target_node_id, e.event_payload, a.created_at
FROM sync_ack_log a
JOIN sync_event_log e ON e.event_id = a.event_id
WHERE a.status = ? AND e.event_payload IS NOT NULL
ORDER BY a.created_at ASC
LIMIT ?
`)).
		WithArgs(StatusPending, 10).
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "target_node_id", "event_payload", "created_at"}).
			AddRow("evt-001", "edge-002", `{"event_id":"evt-001"}`, fixedTime()))

	events, err := New(db).ListPendingReplays(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListPendingReplays returned error: %v", err)
	}
	if len(events) != 1 || events[0].TargetNodeID != "edge-002" || !strings.Contains(string(events[0].Payload), "evt-001") {
		t.Fatalf("unexpected replay events %+v", events)
	}
	assertExpectations(t, mock)
}

func TestStoreMarkRetryPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := New(db)
	store.Clock = fixedTime

	mock.ExpectExec("INSERT INTO sync_ack_log").
		WithArgs("evt-001", "edge-002", StatusPending, nil, nil, fixedTime()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.MarkRetryPending(context.Background(), "evt-001", "edge-002"); err != nil {
		t.Fatalf("MarkRetryPending returned error: %v", err)
	}
	assertExpectations(t, mock)
}

func TestStoreUpsertNodeDefaultsActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	store := New(db)
	store.Clock = fixedTime

	mock.ExpectExec("INSERT INTO sync_node_registry").
		WithArgs("edge-001", "Edge 1", "edge", "line-a", StatusActive, fixedTime(), "0.17.0", fixedTime(), fixedTime()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.UpsertNode(context.Background(), NodeRecord{
		NodeID: "edge-001", NodeName: "Edge 1", NodeType: "edge", Location: "line-a", Version: "0.17.0",
	})
	if err != nil {
		t.Fatalf("UpsertNode returned error: %v", err)
	}
	assertExpectations(t, mock)
}

func TestStoreListActiveEdgeNodeIDsFiltersNodes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT node_id, node_name, node_type").
		WillReturnRows(sqlmock.NewRows([]string{"node_id", "node_name", "node_type", "location", "status", "last_heartbeat_at", "version", "created_at", "updated_at"}).
			AddRow("edge-001", "Edge 1", "edge", "", StatusActive, fixedTime(), "0.17.0", fixedTime(), fixedTime()).
			AddRow("edge-002", "Edge 2", "edge", "", StatusDisabled, fixedTime(), "", fixedTime(), fixedTime()).
			AddRow("server-001", "Server", "server", "", StatusActive, fixedTime(), "0.17.0", fixedTime(), fixedTime()))

	nodes, err := New(db).ListActiveEdgeNodeIDs(context.Background())
	if err != nil {
		t.Fatalf("ListActiveEdgeNodeIDs returned error: %v", err)
	}
	if len(nodes) != 1 || nodes[0] != "edge-001" {
		t.Fatalf("unexpected active nodes %+v", nodes)
	}
	assertExpectations(t, mock)
}

func TestStoreUpsertNodeConfigStoresNoSecrets(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	store := New(db)
	store.Clock = fixedTime

	mock.ExpectExec("INSERT INTO sync_node_config").
		WithArgs("edge-001", "127.0.0.1", 3307, "scada_edge", "sync_user", "canal", "scada_edge\\..*", 1000, "edge-001", int64(9), fixedTime()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.UpsertNodeConfig(context.Background(), NodeConfig{
		NodeID: "edge-001", MySQLHost: "127.0.0.1", MySQLPort: 3307, MySQLDatabase: "scada_edge", MySQLUsername: "sync_user",
		CDCType: "canal", CDCFilter: "scada_edge\\..*", CDCBatchSize: 1000, CDCDestination: "edge-001", RuleVersion: 9,
	})
	if err != nil {
		t.Fatalf("UpsertNodeConfig returned error: %v", err)
	}
	assertExpectations(t, mock)
}

func TestStoreValidation(t *testing.T) {
	store := New(nil)
	if err := store.UpsertAck(context.Background(), AckRecord{}); err == nil {
		t.Fatal("expected db error")
	}

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	store = New(db)
	if err := store.UpsertAck(context.Background(), AckRecord{}); err == nil {
		t.Fatal("expected ack validation error")
	}
	if err := store.UpsertDispatch(context.Background(), DispatchRecord{}); err == nil {
		t.Fatal("expected dispatch validation error")
	}
	if err := store.UpsertEventLog(context.Background(), EventLogRecord{}); err == nil {
		t.Fatal("expected event log validation error")
	}
	if err := store.InsertError(context.Background(), ErrorRecord{}); err == nil {
		t.Fatal("expected error log validation error")
	}
	if err := store.UpsertNode(context.Background(), NodeRecord{}); err == nil {
		t.Fatal("expected node validation error")
	}
	if err := store.UpsertNodeConfig(context.Background(), NodeConfig{}); err == nil {
		t.Fatal("expected node config validation error")
	}
}

func assertExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 5, 21, 14, 20, 0, 0, time.UTC)
}

func sampleSyncEvent() event.SyncEvent {
	return event.SyncEvent{
		EventID:      "evt-001",
		EventType:    "UPDATE",
		OriginNodeID: "edge-001",
		SourceNodeID: "edge-001",
		DatabaseName: "scada_edge",
		TableName:    "device_config",
		PrimaryKey: map[string]any{
			"id": 1,
		},
		After: map[string]any{
			"id":    1,
			"name":  "Pump A",
			"value": "ON",
		},
		SchemaVersion: 1,
		CreatedAt:     fixedTime(),
		EventTime:     fixedTime(),
		TraceID:       "trace-001",
	}
}
