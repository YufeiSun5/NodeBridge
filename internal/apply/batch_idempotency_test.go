package apply_test

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
)

func TestSQLWorkerAppendOnlyBatchDeduplicatesEventIDs(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		name := "all-append"
		if mixed {
			name = "append-segment-before-crud"
		}
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			worker := apply.NewSQLWorker(db)
			worker.Clock = fixedClock
			first := appendOnlyMappedEvent("fresh-1", 1)
			second := appendOnlyMappedEvent("fresh-2", 2)
			prior := appendOnlyMappedEvent("prior", 3)
			events := []mapper.MappedEvent{first, second, first, second, prior, prior}
			mock.ExpectBegin()
			if mixed {
				crud := mappedEvent(event.TypeUpdate)
				crud.Event.EventID = "crud"
				events = append(events, crud, first)
				mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id FROM sync_apply_log WHERE event_id IN (?, ?, ?, ?)")).
					WithArgs("fresh-1", "fresh-2", "prior", "crud").
					WillReturnRows(sqlmock.NewRows([]string{"event_id"}).AddRow("prior"))
				mock.ExpectExec("SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
			} else {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id FROM sync_apply_log WHERE event_id IN (?, ?, ?)")).
					WithArgs("fresh-1", "fresh-2", "prior").
					WillReturnRows(sqlmock.NewRows([]string{"event_id"}).AddRow("prior"))
			}
			mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `scada_center`.`alarm_history` (`alarm_message`, `id`, `last_event_id`) VALUES (?, ?, ?), (?, ?, ?)")+"$").
				WithArgs("overheat", int64(1), "fresh-1", "overheat", int64(2), "fresh-2").
				WillReturnResult(sqlmock.NewResult(1, 2))
			mock.ExpectExec("INSERT INTO sync_apply_log").
				WithArgs("fresh-1", "edge-001", "edge-001", "server-001", "scada_edge", "alarm_history", "scada_center", "alarm_history", `{"id":1}`, event.TypeInsert, fixedClock(),
					"fresh-2", "edge-001", "edge-001", "server-001", "scada_edge", "alarm_history", "scada_center", "alarm_history", `{"id":2}`, event.TypeInsert, fixedClock()).
				WillReturnResult(sqlmock.NewResult(1, 2))
			if mixed {
				mock.ExpectExec("RELEASE SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("SAVEPOINT nb_apply_6").WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("UPDATE `scada_center`.`device_settings`").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("INSERT INTO sync_apply_log").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("RELEASE SAVEPOINT nb_apply_6").WillReturnResult(sqlmock.NewResult(0, 0))
			}
			mock.ExpectCommit()
			result, err := worker.ApplyBatch(context.Background(), events)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Results) != len(events) {
				t.Fatalf("one result per delivery required: %+v", result)
			}
			for i, actual := range result.Results {
				wantDuplicate := i >= 2 && i != 6
				if actual.EventID != events[i].Event.EventID || actual.AlreadyApplied != wantDuplicate {
					t.Fatalf("result %d: %+v, duplicate=%v", i, actual, wantDuplicate)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSQLWorkerCompactReplayCannotRestoreOlderValue(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	worker := apply.NewSQLWorker(db)
	worker.Clock = fixedClock
	first := compactMappedEvent("update-1", 7, "v1")
	second := compactMappedEvent("update-2", 7, "v2")
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id FROM sync_apply_log WHERE event_id IN (?, ?)")).
		WithArgs("update-1", "update-2").WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	mock.ExpectExec("SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE `scada_center`.`device_settings`").
		WithArgs("pump-a", "update-2", "v2", int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WithArgs("update-1", "edge-001", "edge-001", "server-001", "scada_edge", "device_config", "scada_center", "device_settings", `{"setting_id":7}`, event.TypeUpdate, fixedClock(),
			"update-2", "edge-001", "edge-001", "server-001", "scada_edge", "device_config", "scada_center", "device_settings", `{"setting_id":7}`, event.TypeUpdate, fixedClock()).
		WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	result, err := worker.ApplyBatch(context.Background(), []mapper.MappedEvent{first, second, first})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 3 || result.Results[0].AlreadyApplied || result.Results[1].AlreadyApplied || !result.Results[2].AlreadyApplied {
		t.Fatalf("unexpected results: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLWorkerDistinctEventsWithSameBusinessKeyStillFail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	worker := apply.NewSQLWorker(db)
	first := appendOnlyMappedEvent("evt-1", 1)
	second := appendOnlyMappedEvent("evt-2", 1)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id FROM sync_apply_log WHERE event_id IN (?, ?)")).
		WithArgs("evt-1", "evt-2").WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	collision := errors.New("Error 1062: duplicate business primary key")
	mock.ExpectExec("INSERT INTO `scada_center`.`alarm_history`").
		WithArgs("overheat", int64(1), "evt-1", "overheat", int64(1), "evt-2").WillReturnError(collision)
	mock.ExpectRollback()
	result, err := worker.ApplyBatch(context.Background(), []mapper.MappedEvent{first, second, first})
	if !errors.Is(err, collision) || len(result.Results) != 0 {
		t.Fatalf("collision must roll back without ACK results: %+v, %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLWorkerFailedDuplicateSegmentReturnsOnlyCommittedPrefix(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	worker := apply.NewSQLWorker(db)
	first := mappedEvent(event.TypeUpdate)
	second := appendOnlyMappedEvent("append", 2)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id FROM sync_apply_log WHERE event_id IN (?, ?)")).
		WithArgs("evt-001", "append").WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	mock.ExpectExec("SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE `scada_center`.`device_settings`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SAVEPOINT nb_apply_1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO `scada_center`.`alarm_history`").
		WithArgs("overheat", int64(2), "append").WillReturnResult(sqlmock.NewResult(1, 1))
	logFailure := errors.New("apply log write failed")
	mock.ExpectExec("INSERT INTO sync_apply_log").WillReturnError(logFailure)
	mock.ExpectExec("ROLLBACK TO SAVEPOINT nb_apply_1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	result, err := worker.ApplyBatch(context.Background(), []mapper.MappedEvent{first, second, second})
	if !errors.Is(err, logFailure) || len(result.Results) != 1 || result.Results[0].EventID != "evt-001" {
		t.Fatalf("failed segment must not leak results: %+v, %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
