package syncstore

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func batchLogRecords(count int) []EventLogRecord {
	records := make([]EventLogRecord, count)
	for i := range records {
		evt := sampleSyncEvent()
		evt.EventID = fmt.Sprintf("batch-event-%d", i)
		records[i] = EventLogRecord{Event: evt, Status: StatusPending, Direction: "BIDIRECTIONAL", PKValue: "1"}
	}
	return records
}

func TestEventLogBatchBoundaries(t *testing.T) {
	for _, count := range []int{0, 1, maxEventLogBatchRows, maxEventLogBatchRows + 1, 2*maxEventLogBatchRows + 1} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if count > 0 {
				mock.ExpectBegin()
				for remaining := count; remaining > 0; {
					n := min(remaining, maxEventLogBatchRows)
					mock.ExpectExec(regexp.QuoteMeta(eventLogBatchUpsertSQL(n))).WillReturnResult(sqlmock.NewResult(1, int64(n)))
					remaining -= n
				}
				mock.ExpectCommit()
			}
			if err := New(db).UpsertEventLogs(context.Background(), batchLogRecords(count)); err != nil {
				t.Fatal(err)
			}
			assertExpectations(t, mock)
		})
	}
}

func TestEventLogBatchPayloadBudget(t *testing.T) {
	for _, size := range []int{maxEventLogBatchBytes / 3, maxEventLogBatchBytes * 2} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			records := batchLogRecords(2)
			for i := range records {
				records[i].Payload = []byte(strings.Repeat("x", size))
			}
			mock.ExpectBegin()
			for range records {
				mock.ExpectExec(regexp.QuoteMeta(eventLogBatchUpsertSQL(1))).WillReturnResult(sqlmock.NewResult(1, 1))
			}
			mock.ExpectCommit()
			if err := New(db).UpsertEventLogs(context.Background(), records); err != nil {
				t.Fatal(err)
			}
			assertExpectations(t, mock)
		})
	}
}

func TestEventLogBatchFailureRollsBackEveryChunk(t *testing.T) {
	for _, failure := range []string{"begin", "first_chunk", "second_chunk", "late_invalid", "late_encode", "commit"} {
		t.Run(failure, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			records := batchLogRecords(maxEventLogBatchRows + 2)
			cause := errors.New("injected database failure")
			begin := mock.ExpectBegin()
			if failure == "begin" {
				begin.WillReturnError(cause)
			} else {
				first := mock.ExpectExec(regexp.QuoteMeta(eventLogBatchUpsertSQL(maxEventLogBatchRows)))
				if failure == "first_chunk" {
					first.WillReturnError(cause)
				} else {
					first.WillReturnResult(sqlmock.NewResult(1, maxEventLogBatchRows))
					switch failure {
					case "late_invalid":
						records[len(records)-1].Status = ""
					case "late_encode":
						records[len(records)-1].Event.After = map[string]any{"invalid": make(chan int)}
					default:
						last := mock.ExpectExec(regexp.QuoteMeta(eventLogBatchUpsertSQL(2)))
						if failure == "second_chunk" {
							last.WillReturnError(cause)
						} else {
							last.WillReturnResult(sqlmock.NewResult(1, 2))
						}
					}
				}
				if failure == "commit" {
					mock.ExpectCommit().WillReturnError(cause)
				} else {
					mock.ExpectRollback()
				}
			}
			err = New(db).UpsertEventLogs(context.Background(), records)
			if err == nil || (!strings.HasPrefix(failure, "late_") && !errors.Is(err, cause)) {
				t.Fatalf("failure not propagated: %v", err)
			}
			assertExpectations(t, mock)
		})
	}
}

func TestEventLogBatchCancellationAndMissingDB(t *testing.T) {
	if err := New(nil).UpsertEventLogs(context.Background(), batchLogRecords(1)); err == nil {
		t.Fatal("nil DB accepted")
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := New(db).UpsertEventLogs(ctx, batchLogRecords(1)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not preserved: %v", err)
	}
	assertExpectations(t, mock)
}

func TestEventLogBatchSQLShapeAndPayloadOptions(t *testing.T) {
	for _, n := range []int{1, maxEventLogBatchRows} {
		query := eventLogBatchUpsertSQL(n)
		if strings.Count(query, "?") != n*eventLogColumnCount || strings.Count(query, "ON DUPLICATE KEY UPDATE") != 1 {
			t.Fatalf("invalid SQL shape for %d rows", n)
		}
	}
	s := New(nil)
	s.Clock = fixedTime
	r := batchLogRecords(1)[0]
	r.Payload = []byte("supplied-payload")
	args, err := s.eventLogArgs(r)
	if err != nil || args[15] != "supplied-payload" || args[12] != fixedTime() {
		t.Fatalf("payload or timestamp changed: %v %v", args, err)
	}
	r.SkipPayload = true
	args, err = s.eventLogArgs(r)
	if err != nil || args[15] != nil {
		t.Fatalf("SkipPayload changed: %v %v", args, err)
	}
}
