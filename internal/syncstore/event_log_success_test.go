package syncstore

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestEventLogSuccessChunksAndDuplicates(t *testing.T) {
	for _, count := range []int{0, 1, 128, 129, 257} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var ids []string
			for i := 0; i < count; i++ {
				ids = append(ids, fmt.Sprint(i))
			}
			if count > 0 {
				ids = append(ids, ids[0])
				mock.ExpectBegin()
				for remaining := count; remaining > 0; {
					n := min(remaining, maxEventLogBatchRows)
					mock.ExpectExec("^UPDATE sync_event_log SET status=\\?, applied_at=\\?, error_message=NULL WHERE event_id IN").WillReturnResult(sqlmock.NewResult(0, int64(n)))
					remaining -= n
				}
				mock.ExpectCommit()
			}
			if err := New(db).MarkEventLogsSucceeded(context.Background(), ids, fixedTime()); err != nil {
				t.Fatal(err)
			}
			assertExpectations(t, mock)
		})
	}
}

func TestEventLogSuccessNoChangeChecksPresence(t *testing.T) {
	for _, present := range []int{0, 1} {
		t.Run(fmt.Sprint(present), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			s := New(db)
			s.Clock = fixedTime
			mock.ExpectBegin()
			mock.ExpectExec("^UPDATE sync_event_log").WithArgs(StatusSuccess, fixedTime(), "e").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery("^SELECT COUNT").WithArgs(StatusSuccess, "e").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(present))
			if present == 0 {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			err = s.MarkEventLogsSucceeded(context.Background(), []string{"e"}, time.Time{})
			if (err == nil) != (present == 1) {
				t.Fatalf("present=%d err=%v", present, err)
			}
			assertExpectations(t, mock)
		})
	}
}

func TestEventLogSuccessFailures(t *testing.T) {
	for _, failure := range []string{"begin", "exec", "rows", "query", "commit", "second_chunk"} {
		t.Run(failure, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			cause := errors.New("injected success write failure")
			ids := []string{"one"}
			if failure == "second_chunk" {
				for i := 1; i < 129; i++ {
					ids = append(ids, fmt.Sprint(i))
				}
			}
			if failure == "begin" {
				mock.ExpectBegin().WillReturnError(cause)
			} else {
				mock.ExpectBegin()
				exec := mock.ExpectExec("^UPDATE sync_event_log")
				switch failure {
				case "exec":
					exec.WillReturnError(cause)
				case "rows":
					exec.WillReturnResult(sqlmock.NewErrorResult(cause))
				case "query":
					exec.WillReturnResult(sqlmock.NewResult(0, 0))
					mock.ExpectQuery("^SELECT COUNT").WillReturnError(cause)
				case "second_chunk":
					exec.WillReturnResult(sqlmock.NewResult(0, 128))
					mock.ExpectExec("^UPDATE sync_event_log").WillReturnError(cause)
				default:
					exec.WillReturnResult(sqlmock.NewResult(0, 1))
				}
				if failure == "commit" {
					mock.ExpectCommit().WillReturnError(cause)
				} else {
					mock.ExpectRollback()
				}
			}
			if err := New(db).MarkEventLogsSucceeded(context.Background(), ids, fixedTime()); !errors.Is(err, cause) {
				t.Fatalf("lost cause: %v", err)
			}
			assertExpectations(t, mock)
		})
	}
}

func TestEventLogSuccessInvalidAndCanceled(t *testing.T) {
	if New(nil).MarkEventLogsSucceeded(context.Background(), []string{"e"}, fixedTime()) == nil {
		t.Fatal("nil db accepted")
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if New(db).MarkEventLogsSucceeded(context.Background(), []string{""}, fixedTime()) == nil {
		t.Fatal("empty ID accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := New(db).MarkEventLogsSucceeded(ctx, []string{"e"}, fixedTime()); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	assertExpectations(t, mock)
}
