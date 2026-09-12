package dbgovernance_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/dbgovernance"
)

func TestQueryUsesStructuredIdentifiersFiltersAndLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := dbgovernance.New(db, "scada_edge")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id`, `name` FROM `scada_edge`.`device_config` WHERE `id` >= ? AND `name` LIKE ? ORDER BY `id` DESC LIMIT 10")).
		WithArgs(float64(7), "Pump%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(7), []byte("Pump A")))
	result, err := service.Query(context.Background(), dbgovernance.QueryRequest{
		Table: "device_config", Columns: []string{"id", "name"}, Limit: 10,
		Filters: []dbgovernance.Filter{{Column: "id", Operator: ">=", Value: float64(7)}, {Column: "name", Operator: "LIKE", Value: "Pump%"}},
		OrderBy: []dbgovernance.Order{{Column: "id", Direction: "DESC"}},
	})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if result.Count != 1 || result.Rows[0]["name"] != "Pump A" || result.Rows[0]["id"] != int64(7) {
		t.Fatalf("unexpected query result %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQueryRejectsIdentifierInjectionAndOversizedLimit(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	service := dbgovernance.New(db, "scada_edge")
	if _, err := service.Query(context.Background(), dbgovernance.QueryRequest{Table: "device_config; DROP TABLE x"}); err == nil {
		t.Fatal("expected unsafe table to be rejected")
	}
	if _, err := service.Query(context.Background(), dbgovernance.QueryRequest{Table: "device_config", Limit: 201}); err == nil {
		t.Fatal("expected oversized limit to be rejected")
	}
}

func TestMutationPlanCountsRowsWithoutWriting(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	service := dbgovernance.New(db, "scada_edge")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `scada_edge`.`device_config` WHERE `id` = ?")).
		WithArgs(float64(8)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	plan, err := service.PlanMutation(context.Background(), dbgovernance.MutationRequest{
		Operation: "UPDATE", Table: "device_config", Values: map[string]any{"value": "AUTO"},
		Filters: []dbgovernance.Filter{{Column: "id", Operator: "=", Value: float64(8)}},
	})
	if err != nil {
		t.Fatalf("PlanMutation returned error: %v", err)
	}
	if plan.MatchedRows != 1 || plan.Statement != "UPDATE `scada_edge`.`device_config` SET `value` = ? WHERE `id` = ?" {
		t.Fatalf("unexpected plan %+v", plan)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMutationApplyRequiresStableExpectedRowsAndCommits(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	service := dbgovernance.New(db, "scada_edge")
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `scada_edge`.`device_config` WHERE `id` = ?")).
		WithArgs(float64(8)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `scada_edge`.`device_config` SET `name` = ?, `value` = ? WHERE `id` = ? LIMIT 1")).
		WithArgs("Pump 8", "AUTO", float64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	result, err := service.ApplyMutation(context.Background(), dbgovernance.MutationRequest{
		Operation: "UPDATE", Table: "device_config", Values: map[string]any{"value": "AUTO", "name": "Pump 8"},
		Filters: []dbgovernance.Filter{{Column: "id", Operator: "=", Value: float64(8)}}, ExpectedRows: 1, Confirm: true,
	})
	if err != nil {
		t.Fatalf("ApplyMutation returned error: %v", err)
	}
	if !result.OK || result.AffectedRows != 1 {
		t.Fatalf("unexpected mutation result %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMutationApplyRollsBackWhenMatchedRowsChanged(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	service := dbgovernance.New(db, "scada_edge")
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `scada_edge`.`device_config` WHERE `name` LIKE ?")).
		WithArgs("Pump%").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectRollback()
	_, err := service.ApplyMutation(context.Background(), dbgovernance.MutationRequest{
		Operation: "DELETE", Table: "device_config",
		Filters: []dbgovernance.Filter{{Column: "name", Operator: "LIKE", Value: "Pump%"}}, ExpectedRows: 1, Confirm: true,
	})
	if err == nil || !strings.Contains(err.Error(), "matched rows changed") {
		t.Fatalf("expected stable-row rejection, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMutationRejectsUnfilteredAndInternalTableWrites(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	service := dbgovernance.New(db, "scada_edge")
	if _, err := service.PlanMutation(context.Background(), dbgovernance.MutationRequest{Operation: "DELETE", Table: "device_config"}); err == nil {
		t.Fatal("expected unfiltered delete rejection")
	}
	if _, err := service.PlanMutation(context.Background(), dbgovernance.MutationRequest{Operation: "UPDATE", Table: "sync_apply_log", Values: map[string]any{"event_id": "x"}, Filters: []dbgovernance.Filter{{Column: "event_id", Operator: "=", Value: "x"}}}); err == nil {
		t.Fatal("expected internal table rejection")
	}
}

func TestSchemaChangePlanAndApplyAddColumn(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	service := dbgovernance.New(db, "scada_edge")
	change := dbgovernance.SchemaChangeRequest{
		Operation: "ADD_COLUMN", Table: "device_config",
		Column: dbgovernance.ColumnDefinition{Name: "governed_note", Type: "varchar(64)", Nullable: true, Default: dbgovernance.ColumnDefault{Mode: "literal", Value: "new"}, Comment: "MCP governed"},
	}
	expectMissingColumn(mock, "scada_edge", "device_config", "governed_note")
	plan, err := service.PlanSchemaChange(context.Background(), change)
	if err != nil {
		t.Fatalf("PlanSchemaChange returned error: %v", err)
	}
	want := "ALTER TABLE `scada_edge`.`device_config` ADD COLUMN `governed_note` VARCHAR(64) NULL DEFAULT 'new' COMMENT 'MCP governed'"
	if plan.Statement != want || plan.PlanID == "" || plan.Status != "ready" {
		t.Fatalf("unexpected schema plan %+v", plan)
	}
	expectMissingColumn(mock, "scada_edge", "device_config", "governed_note")
	mock.ExpectExec(regexp.QuoteMeta(want)).WillReturnResult(sqlmock.NewResult(0, 0))
	result, err := service.ApplySchemaChange(context.Background(), dbgovernance.SchemaChangeApplyRequest{Change: change, PlanID: plan.PlanID, Confirm: true})
	if err != nil {
		t.Fatalf("ApplySchemaChange returned error: %v", err)
	}
	if !result.OK || result.Status != "applied" {
		t.Fatalf("unexpected schema result %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaDropRejectsIndexedColumn(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	service := dbgovernance.New(db, "scada_edge")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?")).
		WithArgs("scada_edge", "device_config").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?")).
		WithArgs("scada_edge", "device_config", "lookup_code").WillReturnRows(sqlmock.NewRows([]string{"type", "nullable", "key"}).AddRow("varchar(32)", "YES", ""))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?")).
		WithArgs("scada_edge", "device_config", "lookup_code").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	_, err := service.PlanSchemaChange(context.Background(), dbgovernance.SchemaChangeRequest{Operation: "DROP_COLUMN", Table: "device_config", Column: dbgovernance.ColumnDefinition{Name: "lookup_code"}})
	if err == nil || !strings.Contains(err.Error(), "indexed") {
		t.Fatalf("expected indexed-column rejection, got %v", err)
	}
}

func TestSchemaChangeNeverCreatesMissingTable(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	service := dbgovernance.New(db, "scada_edge")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?")).
		WithArgs("scada_edge", "missing_table").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	_, err := service.PlanSchemaChange(context.Background(), dbgovernance.SchemaChangeRequest{Operation: "ADD_COLUMN", Table: "missing_table", Column: dbgovernance.ColumnDefinition{Name: "note", Type: "text", Nullable: true}})
	if err == nil || !strings.Contains(err.Error(), "table creation is disabled") {
		t.Fatalf("expected missing-table rejection, got %v", err)
	}
}

func TestQueryNormalizesTimeValues(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	service := dbgovernance.New(db, "scada_edge")
	stamp := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `scada_edge`.`device_config` LIMIT 1")).
		WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(stamp))
	result, err := service.Query(context.Background(), dbgovernance.QueryRequest{Table: "device_config", Limit: 1})
	if err != nil || result.Rows[0]["updated_at"] != stamp.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected result=%+v err=%v", result, err)
	}
}

func expectMissingColumn(mock sqlmock.Sqlmock, database, table, column string) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?")).
		WithArgs(database, table).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?")).
		WithArgs(database, table, column).WillReturnRows(sqlmock.NewRows([]string{"type", "nullable", "key"}))
}
