package syncstore

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDeleteReplayReceiptLookup(t *testing.T) {
	for _, mode := range []string{"found", "missing", "query_error", "commit_error"} {
		t.Run(mode, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			query := mock.ExpectQuery("SELECT 1 FROM sync_delete_replay .* FOR SHARE").WithArgs("event", "origin", "db", "table")
			if mode == "query_error" {
				query.WillReturnError(errors.New("read failure"))
				mock.ExpectRollback()
			} else {
				rows := sqlmock.NewRows([]string{"1"})
				if mode != "missing" {
					rows.AddRow(1)
				}
				query.WillReturnRows(rows)
				commit := mock.ExpectCommit()
				if mode == "commit_error" {
					commit.WillReturnError(errors.New("commit failure"))
				}
			}
			found, err := New(db).IsDeleteReplay(context.Background(), "event", "origin", "db", "table")
			if found != (mode == "found") || (err != nil) != (mode == "query_error" || mode == "commit_error") {
				t.Fatalf("found=%t err=%v", found, err)
			}
			assertExpectations(t, mock)
		})
	}
}
