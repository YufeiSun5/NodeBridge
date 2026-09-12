package syncstore

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestReplayLookupTransactionAndErrors(t *testing.T) {
	for _, mode := range []string{"found", "missing", "begin_error", "read_error", "commit_error"} {
		t.Run(mode, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			failure := errors.New("replay lookup unavailable")
			begin := mock.ExpectBegin()
			if mode == "begin_error" {
				begin.WillReturnError(failure)
			} else {
				query := mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM sync_apply_log WHERE event_id = ? FOR SHARE")).WithArgs("evt-replayed")
				if mode == "read_error" {
					query.WillReturnError(failure)
					mock.ExpectRollback()
				} else {
					rows := sqlmock.NewRows([]string{"1"})
					if mode != "missing" {
						rows.AddRow(1)
					}
					query.WillReturnRows(rows)
					commit := mock.ExpectCommit()
					if mode == "commit_error" {
						commit.WillReturnError(failure)
					}
				}
			}
			found, err := New(db).Exists(context.Background(), "evt-replayed")
			wantError := mode != "found" && mode != "missing"
			if wantError && !errors.Is(err, failure) || !wantError && err != nil || found != (mode == "found") {
				t.Fatalf("mode=%s found=%t err=%v", mode, found, err)
			}
			assertExpectations(t, mock)
		})
	}
}

func TestReplayLookupRequiresDatabaseAndPropagatesCancellation(t *testing.T) {
	if found, err := New(nil).Exists(context.Background(), "evt"); err == nil || found {
		t.Fatalf("missing database accepted: found=%t err=%v", found, err)
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if found, err := New(db).Exists(context.Background(), ""); err != nil || found {
		t.Fatalf("empty event lookup: found=%t err=%v", found, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if found, err := New(db).Exists(ctx, "evt"); !errors.Is(err, context.Canceled) || found {
		t.Fatalf("cancellation lost: found=%t err=%v", found, err)
	}
	assertExpectations(t, mock)
}
