package main

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUpdateRequiresExistingRow(t *testing.T) {
	for _, affected := range []int64{0, 1, 2} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		l := lab{s: fixture(t)}
		mock.ExpectBegin()
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectExec("UPDATE `test` SET `revision`=\\? WHERE `id`=\\?").WithArgs(int64(2), int64(5)).WillReturnResult(sqlmock.NewResult(0, affected))
		row := makeRow(settings{}, layouts("nb_unit")[0], 5, 2)
		err = l.update(context.Background(), tx, table{"test", nil}, row, []int{3})
		if (err == nil) != (affected == 1) {
			t.Fatalf("affected %d: %v", affected, err)
		}
		mock.ExpectRollback()
		_ = tx.Rollback()
		if err = mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		_ = db.Close()
	}
}

type jsonText struct{}

func (jsonText) Match(value driver.Value) bool { _, ok := value.(string); return ok }

func TestOracleBatchUsesTextInsideBusinessTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	l := lab{c: settings{Prefix: "nb_unit"}, s: fixture(t)}
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("INSERT INTO `nb_unit_oracle`").WithArgs(int64(1), int64(2), jsonText{}, int64(2), int64(3), jsonText{}).WillReturnResult(sqlmock.NewResult(0, 2))
	err = l.saveOracleBatch(context.Background(), tx, [][]any{makeRow(l.c, layouts("nb_unit")[0], 1, 2), makeRow(l.c, layouts("nb_unit")[0], 2, 3)})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
