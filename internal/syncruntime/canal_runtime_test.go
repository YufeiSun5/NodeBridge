package syncruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
)

func TestCanalUploadRuntimePublishesThenCommits(t *testing.T) {
	source := &fakeCanalBatchSource{
		changes: []cdc.ChangeEvent{sampleChange()},
		offset:  cdc.Offset{ReaderName: "edge-001", BatchID: 10, BinlogFile: "mysql-bin.000001"},
	}
	publisher := &fakePublisher{}
	runtime := &CanalUploadRuntime{
		Source:     source,
		Decider:    fakeDecider{decision: loop.Decision{Upload: true}},
		Normalizer: fakeNormalizer{event: sampleEvent()},
		Publisher:  publisher,
		Exchange:   "edge.upload.x",
		RoutingKey: "edge.upload.cdc",
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "published" || result.EventID != "evt-001" || result.DispatchCount != 1 {
		t.Fatalf("unexpected result %+v", result)
	}
	if !source.started || !source.committed {
		t.Fatalf("expected start and commit, got %+v", source)
	}
	if len(publisher.requests) != 1 {
		t.Fatalf("expected one publish, got %d", len(publisher.requests))
	}
}

func TestCanalUploadRuntimeDoesNotCommitOnPublishFailure(t *testing.T) {
	source := &fakeCanalBatchSource{
		changes: []cdc.ChangeEvent{sampleChange()},
		offset:  cdc.Offset{ReaderName: "edge-001", BatchID: 10, BinlogFile: "mysql-bin.000001"},
	}
	runtime := &CanalUploadRuntime{
		Source:     source,
		Normalizer: fakeNormalizer{event: sampleEvent()},
		Publisher:  &fakePublisher{err: errors.New("broker down")},
	}

	result, err := runtime.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected publish error")
	}
	if result.Action != "failed" || source.committed {
		t.Fatalf("unexpected result=%+v committed=%t", result, source.committed)
	}
}

func TestCanalUploadRuntimeSuppressesAndCommits(t *testing.T) {
	source := &fakeCanalBatchSource{changes: []cdc.ChangeEvent{sampleChange()}}
	result, err := (&CanalUploadRuntime{
		Source:     source,
		Decider:    fakeDecider{decision: loop.Decision{Upload: false}},
		Normalizer: fakeNormalizer{event: sampleEvent()},
		Publisher:  &fakePublisher{},
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "suppressed" || !source.committed {
		t.Fatalf("unexpected result=%+v committed=%t", result, source.committed)
	}
}

func TestCanalUploadRuntimeStop(t *testing.T) {
	source := &fakeCanalBatchSource{}
	runtime := &CanalUploadRuntime{
		Source:     source,
		Normalizer: fakeNormalizer{event: event.SyncEvent{EventID: "evt-001"}},
		Publisher:  &fakePublisher{},
	}
	if _, err := runtime.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if err := runtime.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if !source.stopped {
		t.Fatal("expected source stopped")
	}
}

type fakeCanalBatchSource struct {
	changes   []cdc.ChangeEvent
	offset    cdc.Offset
	err       error
	started   bool
	stopped   bool
	committed bool
}

func (s *fakeCanalBatchSource) Start(ctx context.Context) error {
	s.started = true
	return s.err
}

func (s *fakeCanalBatchSource) Stop(ctx context.Context) error {
	s.stopped = true
	return s.err
}

func (s *fakeCanalBatchSource) FetchChangesOnce(ctx context.Context) ([]cdc.ChangeEvent, cdc.Offset, error) {
	return s.changes, s.offset, s.err
}

func (s *fakeCanalBatchSource) Commit(ctx context.Context, offset cdc.Offset) error {
	s.committed = true
	return s.err
}
