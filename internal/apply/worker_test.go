package apply_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
)

func TestSQLWorkerApplyInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	worker := apply.NewSQLWorker(db)
	worker.Clock = fixedClock
	mapped := mappedEvent(event.TypeInsert)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(1) FROM sync_apply_log WHERE event_id = ?")).
		WithArgs("evt-001").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO `scada_center`.`device_settings`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WithArgs("evt-001", "edge-001", "edge-001", "server-001", "scada_edge", "device_config", "scada_center", "device_settings", `{"setting_id":7}`, event.TypeInsert, fixedClock()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := worker.Apply(context.Background(), mapped)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if result.AlreadyApplied {
		t.Fatal("expected fresh apply")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLWorkerSkipsDuplicateEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	worker := apply.NewSQLWorker(db)
	mapped := mappedEvent(event.TypeUpdate)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(1) FROM sync_apply_log WHERE event_id = ?")).
		WithArgs("evt-001").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectCommit()

	result, err := worker.Apply(context.Background(), mapped)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !result.AlreadyApplied {
		t.Fatal("expected duplicate event to be skipped")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLWorkerApplyUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	worker := apply.NewSQLWorker(db)
	worker.Clock = fixedClock
	mapped := mappedEvent(event.TypeUpdate)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(1) FROM sync_apply_log WHERE event_id = ?")).
		WithArgs("evt-001").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("UPDATE `scada_center`.`device_settings` SET").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if _, err := worker.Apply(context.Background(), mapped); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLWorkerApplySoftDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	worker := apply.NewSQLWorker(db)
	worker.Clock = fixedClock
	mapped := mappedEvent(event.TypeDelete)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(1) FROM sync_apply_log WHERE event_id = ?")).
		WithArgs("evt-001").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("UPDATE `scada_center`.`device_settings` SET `is_deleted`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if _, err := worker.Apply(context.Background(), mapped); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func mappedEvent(eventType string) mapper.MappedEvent {
	evt := event.SyncEvent{
		EventID:      "evt-001",
		EventType:    eventType,
		OriginNodeID: "edge-001",
		SourceNodeID: "edge-001",
		TargetNodeID: "server-001",
	}
	return mapper.MappedEvent{
		Event:          evt,
		SourceDatabase: "scada_edge",
		SourceTable:    "device_config",
		TargetDatabase: "scada_center",
		TargetTable:    "device_settings",
		TargetPrimaryKey: map[string]any{
			"setting_id": int64(7),
		},
		TargetAfter: map[string]any{
			"setting_id":    int64(7),
			"display_name":  "pump-a",
			"setting_value": "new",
			"last_event_id": "evt-001",
		},
	}
}

func fixedClock() time.Time {
	return time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
}
