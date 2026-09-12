package main

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLifecycleTransactionBoundaries(t *testing.T) {
	for _, mode := range []string{"insert", "update", "reinsert_oracle", "write_error", "oracle_error", "commit_error", "begin_error"} {
		t.Run(mode, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			l := lab{c: settings{RunID: "unit", Prefix: "nb_unit"}, s: fixture(t)}
			l.db[0] = db
			target := layouts("nb_unit")[0]
			row := makeRow(l.c, target, 10, 5)
			begin := mock.ExpectBegin()
			if mode == "begin_error" {
				begin.WillReturnError(errors.New("begin failed"))
			} else {
				query := "INSERT INTO"
				if mode == "update" {
					query = "UPDATE"
				}
				write := mock.ExpectExec(query)
				if mode == "write_error" {
					write.WillReturnError(errors.New("write failed"))
					mock.ExpectRollback()
				} else {
					write.WillReturnResult(sqlmock.NewResult(1, 1))
					if mode == "reinsert_oracle" || mode == "oracle_error" {
						oracle := mock.ExpectExec("INSERT INTO `nb_unit_oracle`")
						if mode == "oracle_error" {
							oracle.WillReturnError(errors.New("oracle failed"))
							mock.ExpectRollback()
						} else {
							oracle.WillReturnResult(sqlmock.NewResult(1, 1))
						}
					}
					if mode != "oracle_error" {
						commit := mock.ExpectCommit()
						if mode == "commit_error" {
							commit.WillReturnError(errors.New("commit failed"))
						}
					}
				}
			}
			err = l.commitLifecycleRow(0, target.State, row, mode != "update", mode == "reinsert_oracle" || mode == "oracle_error")
			wantError := mode != "insert" && mode != "update" && mode != "reinsert_oracle"
			if (err != nil) != wantError {
				t.Fatalf("error=%v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
