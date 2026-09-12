package apply_test

import (
	"context"
	"database/sql/driver"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/dbgovernance"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
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

func TestSQLWorkerApplyBatchPreservesOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	first := mappedEvent(event.TypeInsert)
	second := mappedEvent(event.TypeInsert)
	second.Event.EventID = "evt-002"
	second.TargetPrimaryKey = map[string]any{"setting_id": 2}
	second.TargetAfter["setting_id"] = 2

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id FROM sync_apply_log WHERE event_id IN (?, ?)")).
		WithArgs("evt-001", "evt-002").
		WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	mock.ExpectExec("SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO `scada_center`.`device_settings`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SAVEPOINT nb_apply_1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO `scada_center`.`device_settings`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	result, err := apply.NewSQLWorker(db).ApplyBatch(context.Background(), []mapper.MappedEvent{first, second})
	if err != nil {
		t.Fatalf("ApplyBatch returned error: %v", err)
	}
	if len(result.Results) != 2 || result.Results[0].EventID != "evt-001" || result.Results[1].EventID != "evt-002" {
		t.Fatalf("unexpected batch result %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLWorkerApplyBatchCommitsPrefixOnFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	first := mappedEvent(event.TypeInsert)
	second := mappedEvent(event.TypeInsert)
	second.Event.EventID = "evt-002"
	second.TargetPrimaryKey = map[string]any{"setting_id": 2}
	second.TargetAfter["setting_id"] = 2

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id FROM sync_apply_log WHERE event_id IN (?, ?)")).
		WithArgs("evt-001", "evt-002").
		WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	mock.ExpectExec("SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO `scada_center`.`device_settings`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SAVEPOINT nb_apply_1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO `scada_center`.`device_settings`").WillReturnError(context.Canceled)
	mock.ExpectExec("ROLLBACK TO SAVEPOINT nb_apply_1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	result, err := apply.NewSQLWorker(db).ApplyBatch(context.Background(), []mapper.MappedEvent{first, second})
	if err == nil {
		t.Fatal("expected second apply failure")
	}
	if len(result.Results) != 1 || result.Results[0].EventID != "evt-001" {
		t.Fatalf("expected committed prefix only, got %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLWorkerApplyBatchUsesAppendOnlyBulkInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	first := appendOnlyMappedEvent("evt-001", int64(1))
	second := appendOnlyMappedEvent("evt-002", int64(2))
	worker := apply.NewSQLWorker(db)
	worker.Clock = fixedClock

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id FROM sync_apply_log WHERE event_id IN (?, ?)")).
		WithArgs("evt-001", "evt-002").
		WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	mock.ExpectExec("INSERT INTO `scada_center`.`alarm_history`").
		WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectCommit()

	result, err := worker.ApplyBatch(context.Background(), []mapper.MappedEvent{first, second})
	if err != nil {
		t.Fatalf("ApplyBatch returned error: %v", err)
	}
	if len(result.Results) != 2 || result.Results[0].EventID != "evt-001" || result.Results[1].EventID != "evt-002" {
		t.Fatalf("unexpected batch result %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLWorkerApplyBatchDoesNotIgnoreAppendOnlyDataErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	event := appendOnlyMappedEvent("evt-too-long", int64(1))
	worker := apply.NewSQLWorker(db)
	worker.Clock = fixedClock

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id FROM sync_apply_log WHERE event_id IN (?)")).
		WithArgs("evt-too-long").
		WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	mock.ExpectExec("INSERT INTO `scada_center`.`alarm_history`").
		WillReturnError(fmt.Errorf("Error 1406: Data too long for column 'payload'"))
	mock.ExpectRollback()

	result, err := worker.ApplyBatch(context.Background(), []mapper.MappedEvent{event})
	if err == nil {
		t.Fatal("expected append_only data error")
	}
	if len(result.Results) != 0 {
		t.Fatalf("data error must not be reported as applied: %+v", result.Results)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLWorkerApplyBatchGroupsInterleavedAppendOnlyTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	first := appendOnlyMappedEvent("evt-001", int64(1))
	second := appendOnlyMappedEvent("evt-002", int64(2))
	second.SourceTable = "collect_data_02"
	second.TargetTable = "collect_data_02"
	third := appendOnlyMappedEvent("evt-003", int64(3))
	worker := apply.NewSQLWorker(db)
	worker.Clock = fixedClock

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id FROM sync_apply_log WHERE event_id IN (?, ?, ?)")).
		WithArgs("evt-001", "evt-002", "evt-003").
		WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	mock.ExpectExec("INSERT INTO `scada_center`.`alarm_history`").
		WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectExec("INSERT INTO `scada_center`.`collect_data_02`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := worker.ApplyBatch(context.Background(), []mapper.MappedEvent{first, second, third})
	if err != nil {
		t.Fatalf("ApplyBatch returned error: %v", err)
	}
	if len(result.Results) != 3 || result.Results[0].EventID != "evt-001" || result.Results[1].EventID != "evt-002" || result.Results[2].EventID != "evt-003" {
		t.Fatalf("unexpected batch result %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLWorkerApplyBatchGroupsAppendOnlySegmentBeforeCRUD(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	first := appendOnlyMappedEvent("evt-001", int64(1))
	second := appendOnlyMappedEvent("evt-002", int64(2))
	second.SourceTable = "collect_data_02"
	second.TargetTable = "collect_data_02"
	third := mappedEvent(event.TypeUpdate)
	third.Event.EventID = "evt-003"
	fourth := appendOnlyMappedEvent("evt-004", int64(4))
	worker := apply.NewSQLWorker(db)
	worker.Clock = fixedClock

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id FROM sync_apply_log WHERE event_id IN (?, ?, ?, ?)")).
		WithArgs("evt-001", "evt-002", "evt-003", "evt-004").
		WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	mock.ExpectExec("SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO `scada_center`.`alarm_history`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO `scada_center`.`collect_data_02`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SAVEPOINT nb_apply_2").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE `scada_center`.`device_settings`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_2").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SAVEPOINT nb_apply_3").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO `scada_center`.`alarm_history`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_3").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	result, err := worker.ApplyBatch(context.Background(), []mapper.MappedEvent{first, second, third, fourth})
	if err != nil {
		t.Fatalf("ApplyBatch returned error: %v", err)
	}
	if len(result.Results) != 4 {
		t.Fatalf("expected 4 results, got %+v", result)
	}
	for index, want := range []string{"evt-001", "evt-002", "evt-003", "evt-004"} {
		if result.Results[index].EventID != want {
			t.Fatalf("result %d expected %s, got %+v", index, want, result.Results[index])
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLWorkerApplyBatchCompactsConsecutiveUpdatesForSamePrimaryKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	first := compactMappedEvent("evt-001", int64(7), "v1")
	second := compactMappedEvent("evt-002", int64(7), "v2")
	third := compactMappedEvent("evt-003", int64(8), "other")
	worker := apply.NewSQLWorker(db)
	worker.Clock = fixedClock

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id FROM sync_apply_log WHERE event_id IN (?, ?, ?)")).
		WithArgs("evt-001", "evt-002", "evt-003").
		WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	mock.ExpectExec("SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE `scada_center`.`device_settings`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SAVEPOINT nb_apply_2").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE `scada_center`.`device_settings`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_2").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	result, err := worker.ApplyBatch(context.Background(), []mapper.MappedEvent{first, second, third})
	if err != nil {
		t.Fatalf("ApplyBatch returned error: %v", err)
	}
	if len(result.Results) != 3 {
		t.Fatalf("expected 3 results, got %+v", result)
	}
	if result.Results[0].EventID != "evt-001" || result.Results[1].EventID != "evt-002" || result.Results[2].EventID != "evt-003" {
		t.Fatalf("unexpected compact results %+v", result.Results)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLWorkerApplyBatchChunksLargeAppendOnlyBulkInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	events := make([]mapper.MappedEvent, 0, 25001)
	eventIDs := make([]driver.Value, 0, 25001)
	for i := 1; i <= 25001; i++ {
		eventID := fmt.Sprintf("evt-%05d", i)
		events = append(events, appendOnlyMappedEvent(eventID, int64(i)))
		eventIDs = append(eventIDs, eventID)
	}
	worker := apply.NewSQLWorker(db)
	worker.Clock = fixedClock

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT event_id FROM sync_apply_log WHERE event_id IN").
		WithArgs(eventIDs...).
		WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	mock.ExpectExec("INSERT INTO `scada_center`.`alarm_history`").
		WillReturnResult(sqlmock.NewResult(1, 20000))
	mock.ExpectExec("INSERT INTO `scada_center`.`alarm_history`").
		WillReturnResult(sqlmock.NewResult(1, 5001))
	for i := 0; i < 5; i++ {
		mock.ExpectExec("INSERT INTO sync_apply_log").
			WillReturnResult(sqlmock.NewResult(1, 1))
	}
	mock.ExpectCommit()

	result, err := worker.ApplyBatch(context.Background(), events)
	if err != nil {
		t.Fatalf("ApplyBatch returned error: %v", err)
	}
	if len(result.Results) != len(events) {
		t.Fatalf("expected %d results, got %d", len(events), len(result.Results))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLWorkerAppendOnlyRejectsUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mapped := appendOnlyMappedEvent("evt-001", int64(1))
	mapped.Event.EventType = event.TypeUpdate

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(1) FROM sync_apply_log WHERE event_id = ?")).
		WithArgs("evt-001").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectRollback()

	if _, err := apply.NewSQLWorker(db).Apply(context.Background(), mapped); err == nil {
		t.Fatal("expected append_only update to fail")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLWorkerApplySchemaAddColumnAndLog(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mapped := schemaMappedEvent(event.TypeAddColumn, dbgovernance.ColumnDefinition{Name: "governed_note", Type: "varchar(64)", Nullable: true})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(1) FROM sync_apply_log WHERE event_id = ?")).WithArgs("evt-schema-001").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	expectSchemaColumnState(mock, "governed_note", false)
	expectSchemaColumnState(mock, "governed_note", false)
	mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE `scada_center`.`device_settings` ADD COLUMN `governed_note` VARCHAR(64) NULL")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO sync_apply_log").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	worker := apply.NewSQLWorker(db)
	worker.Clock = fixedClock
	result, err := worker.Apply(context.Background(), mapped)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if result.EventID != "evt-schema-001" || result.AlreadyApplied {
		t.Fatalf("unexpected result %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLWorkerSchemaApplyNeverCreatesMissingTable(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	mapped := schemaMappedEvent(event.TypeAddColumn, dbgovernance.ColumnDefinition{Name: "governed_note", Type: "text", Nullable: true})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(1) FROM sync_apply_log WHERE event_id = ?")).WithArgs("evt-schema-001").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?")).
		WithArgs("scada_center", "device_settings").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	if _, err := apply.NewSQLWorker(db).Apply(context.Background(), mapped); err == nil || !strings.Contains(err.Error(), "table creation is disabled") {
		t.Fatalf("expected missing table rejection, got %v", err)
	}
}

func TestSQLWorkerSchemaApplyIsIdempotentFromApplyLog(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	mapped := schemaMappedEvent(event.TypeDropColumn, dbgovernance.ColumnDefinition{Name: "governed_note"})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(1) FROM sync_apply_log WHERE event_id = ?")).WithArgs("evt-schema-001").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	result, err := apply.NewSQLWorker(db).Apply(context.Background(), mapped)
	if err != nil || !result.AlreadyApplied {
		t.Fatalf("unexpected result=%+v err=%v", result, err)
	}
}

func schemaMappedEvent(eventType string, column dbgovernance.ColumnDefinition) mapper.MappedEvent {
	change := &dbgovernance.SchemaChange{Operation: eventType, Column: column}
	return mapper.MappedEvent{
		Event:          event.SyncEvent{EventID: "evt-schema-001", EventType: eventType, OriginNodeID: "edge-001", SourceNodeID: "edge-001", SchemaChange: change},
		SourceDatabase: "scada_edge", SourceTable: "device_config", TargetDatabase: "scada_center", TargetTable: "device_settings",
		SchemaChangeSelected: true,
	}
}

func expectSchemaColumnState(mock sqlmock.Sqlmock, column string, exists bool) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?")).
		WithArgs("scada_center", "device_settings").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	query := mock.ExpectQuery(regexp.QuoteMeta("SELECT COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?")).
		WithArgs("scada_center", "device_settings", column)
	rows := sqlmock.NewRows([]string{"type", "nullable", "key"})
	if exists {
		rows.AddRow("varchar(64)", "YES", "")
	}
	query.WillReturnRows(rows)
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

func appendOnlyMappedEvent(eventID string, id int64) mapper.MappedEvent {
	evt := event.SyncEvent{
		EventID:      eventID,
		EventType:    event.TypeInsert,
		OriginNodeID: "edge-001",
		SourceNodeID: "edge-001",
		TargetNodeID: "server-001",
	}
	return mapper.MappedEvent{
		Event:          evt,
		SourceDatabase: "scada_edge",
		SourceTable:    "alarm_history",
		TargetDatabase: "scada_center",
		TargetTable:    "alarm_history",
		SyncMode:       rules.SyncModeAppendOnly,
		TargetPrimaryKey: map[string]any{
			"id": id,
		},
		TargetAfter: map[string]any{
			"id":            id,
			"alarm_message": "overheat",
			"last_event_id": eventID,
		},
	}
}

func compactMappedEvent(eventID string, id int64, value string) mapper.MappedEvent {
	mapped := mappedEvent(event.TypeUpdate)
	mapped.Event.EventID = eventID
	mapped.SyncMode = rules.SyncModeCRUDCompact
	mapped.TargetPrimaryKey = map[string]any{"setting_id": id}
	mapped.TargetAfter["setting_id"] = id
	mapped.TargetAfter["setting_value"] = value
	mapped.TargetAfter["last_event_id"] = eventID
	return mapped
}

func fixedClock() time.Time {
	return time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
}
