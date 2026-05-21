package syncstore

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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
	if err := store.InsertError(context.Background(), ErrorRecord{}); err == nil {
		t.Fatal("expected error log validation error")
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
