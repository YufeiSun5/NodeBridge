package syncstore

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRepairReplayLookupUsesTransactionAndExactScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT 1 FROM sync_repair_replay .* FOR SHARE").WithArgs("repair", "local", "db", "table", "UPDATE", "UPDATE").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectCommit()
	found, err := New(db).IsRepairReplay(context.Background(), "repair", "local", "db", "table", "UPDATE")
	if err != nil || !found {
		t.Fatalf("%t %v", found, err)
	}
	assertExpectations(t, mock)
}
