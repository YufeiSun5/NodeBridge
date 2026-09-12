package syncruntime

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type recordingSuccessStore struct {
	recordingDispatchStore
	ids []string
	at  time.Time
	err error
}

func (s *recordingSuccessStore) MarkEventLogsSucceeded(_ context.Context, ids []string, at time.Time) error {
	*s.trace = append(*s.trace, "status-update")
	s.ids, s.at = append([]string(nil), ids...), at
	return s.err
}

func TestServerCanalStatusUpdateOrderingAndRetry(t *testing.T) {
	for _, failure := range []string{"", "pending", "publish", "success"} {
		t.Run(failure, func(t *testing.T) {
			var trace []string
			source := newRewindingCanalSource()
			source.trace = &trace
			store := &recordingSuccessStore{recordingDispatchStore: recordingDispatchStore{trace: &trace}}
			dispatcher := &recordingBatchDispatcher{trace: &trace}
			runtime := newBatchDispatchRuntime(source, store, dispatcher)
			cause := errors.New("injected")
			switch failure {
			case "pending":
				store.failStatus = "PENDING"
			case "publish":
				dispatcher.err = cause
			case "success":
				store.err = cause
			}
			_, err := runtime.RunOnce(context.Background())
			if (err == nil) != (failure == "") {
				t.Fatalf("failure=%s err=%v", failure, err)
			}
			want := []string{"PENDING", "publish", "status-update", "commit-1"}
			switch failure {
			case "pending":
				want = want[:1]
			case "publish":
				want = want[:2]
			case "success":
				want = want[:3]
			}
			if !reflect.DeepEqual(trace, want) {
				t.Fatalf("unsafe order: %v", trace)
			}
			if failure == "" || failure == "success" {
				if len(store.ids) != 2 || store.at.IsZero() || len(store.batches) != 1 {
					t.Fatal("expected one full PENDING and status-only completion")
				}
				for i, id := range store.ids {
					if id != store.batches[0][i].Event.EventID {
						t.Fatal("completion IDs changed")
					}
				}
			}
			if failure == "success" {
				store.err = nil
				trace = nil
				if _, err := runtime.RunOnce(context.Background()); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(trace, []string{"PENDING", "publish", "status-update", "commit-1"}) {
					t.Fatalf("failed status did not replay batch: %v", trace)
				}
			}
		})
	}
}
