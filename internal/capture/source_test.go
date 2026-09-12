package capture

import (
	"context"
	"errors"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/syncruntime"
)

type batchSource struct {
	changes   []cdc.ChangeEvent
	offset    cdc.Offset
	err       error
	committed bool
}

func (s *batchSource) Start(context.Context) error { return nil }
func (s *batchSource) Stop(context.Context) error  { return nil }
func (s *batchSource) FetchChangesOnce(context.Context) ([]cdc.ChangeEvent, cdc.Offset, error) {
	return s.changes, s.offset, nil
}
func (s *batchSource) Commit(context.Context, cdc.Offset) error {
	s.committed = s.err == nil
	return s.err
}

type normalizer struct{}

func (normalizer) Normalize(cdc.ChangeEvent) (event.SyncEvent, error) {
	return event.SyncEvent{EventID: "business"}, nil
}

type publisher struct{ err error }

func (p publisher) Publish(context.Context, rabbitmq.PublishRequest) error { return p.err }

func TestSourceBarrierFollowsRuntimeCommit(t *testing.T) {
	for _, stage := range []string{"empty", "business", "publish_failure", "commit_failure"} {
		t.Run(stage, func(t *testing.T) {
			done := make(chan struct{})
			f := &Fence{database: "db", node: "node", pending: map[string]chan struct{}{"token": done}}
			base := &batchSource{changes: []cdc.ChangeEvent{pulse("token")}, offset: cdc.Offset{BatchID: 1}}
			pub := publisher{}
			if stage != "empty" {
				base.changes = append(base.changes, cdc.ChangeEvent{DatabaseName: "db", TableName: "business"})
			}
			if stage == "publish_failure" {
				pub.err = errors.New("no confirm")
			}
			if stage == "commit_failure" {
				base.err = errors.New("no ack")
			}
			source, err := NewSource(base, f)
			if err != nil {
				t.Fatal(err)
			}
			runtime := &syncruntime.CanalUploadRuntime{Source: source, Normalizer: normalizer{}, Publisher: pub}
			_, err = runtime.RunOnce(context.Background())
			failure := stage == "publish_failure" || stage == "commit_failure"
			if (err != nil) != failure {
				t.Fatalf("unexpected error %v", err)
			}
			select {
			case <-done:
				if failure || !base.committed {
					t.Fatal("premature barrier release")
				}
			default:
				if !failure {
					t.Fatal("barrier was not released")
				}
			}
			if failure {
				base.err = nil
				runtime.Publisher = publisher{}
				if _, err := runtime.RunOnce(context.Background()); err != nil {
					t.Fatal(err)
				}
				select {
				case <-done:
				default:
					t.Fatal("retry did not release barrier")
				}
			}
		})
	}
}

func TestSourceRejectsMismatchedAndOutstandingBatches(t *testing.T) {
	f := &Fence{database: "db", node: "node", pending: map[string]chan struct{}{}}
	base := &batchSource{changes: []cdc.ChangeEvent{pulse("token")}, offset: cdc.Offset{BatchID: 4}}
	s, err := NewSource(base, f)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	changes, offset, err := s.FetchChangesOnce(ctx)
	if err != nil || len(changes) != 0 {
		t.Fatalf("pulse forwarded: %v %v", changes, err)
	}
	if _, _, err := s.FetchChangesOnce(ctx); err == nil {
		t.Fatal("fetched past uncommitted batch")
	}
	if err := s.Commit(ctx, cdc.Offset{BatchID: 5}); err == nil || base.committed {
		t.Fatal("accepted wrong batch")
	}
	if err := s.Commit(ctx, offset); err != nil {
		t.Fatal(err)
	}
	base.changes, base.offset.BatchID = nil, -1
	for i := 0; i < 2; i++ {
		if _, _, err := s.FetchChangesOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
}
