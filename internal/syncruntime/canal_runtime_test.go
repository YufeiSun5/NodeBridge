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

func TestCanalUploadRuntimeUsesBatchPublisher(t *testing.T) {
	source := &fakeCanalBatchSource{
		changes: []cdc.ChangeEvent{sampleChange(), sampleChange()},
		offset:  cdc.Offset{ReaderName: "edge-001", BatchID: 10, BinlogFile: "mysql-bin.000001"},
	}
	publisher := &fakeBatchPublisher{}
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
	if result.Action != "published" || result.DispatchCount != 2 {
		t.Fatalf("unexpected result %+v", result)
	}
	if !source.committed {
		t.Fatal("expected canal offset commit after batch publish")
	}
	if len(publisher.batches) != 1 || len(publisher.batches[0]) != 2 {
		t.Fatalf("expected one publish batch with two requests, got %+v", publisher.batches)
	}
	if len(publisher.requests) != 0 {
		t.Fatalf("expected PublishBatch path, got per-message publishes %+v", publisher.requests)
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
	source := &fakeCanalBatchSource{
		changes: []cdc.ChangeEvent{sampleChange()},
		offset:  cdc.Offset{ReaderName: "edge-001", BatchID: 10, BinlogFile: "mysql-bin.000001"},
	}
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

func TestCanalUploadRuntimeCommitsEmptyCanalBatch(t *testing.T) {
	source := &fakeCanalBatchSource{offset: cdc.Offset{ReaderName: "edge-001", BatchID: 12, BinlogFile: "mysql-bin.000003"}}
	result, err := (&CanalUploadRuntime{
		Source:     source,
		Normalizer: fakeNormalizer{event: sampleEvent()},
		Publisher:  &fakePublisher{},
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "committed-empty" || !source.committed {
		t.Fatalf("unexpected result=%+v committed=%t", result, source.committed)
	}
}

func TestCanalUploadRuntimeDoesNotCommitIdleEmptyFetch(t *testing.T) {
	source := &fakeCanalBatchSource{}
	result, err := (&CanalUploadRuntime{
		Source:     source,
		Normalizer: fakeNormalizer{event: sampleEvent()},
		Publisher:  &fakePublisher{},
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "empty" || source.committed {
		t.Fatalf("unexpected result=%+v committed=%t", result, source.committed)
	}
}

func TestCanalUploadRuntimeReconnectsAfterFetchError(t *testing.T) {
	source := &fakeCanalBatchSource{fetchErr: errors.New("canal connection closed")}
	runtime := &CanalUploadRuntime{
		Source:     source,
		Decider:    fakeDecider{decision: loop.Decision{Upload: true}},
		Normalizer: fakeNormalizer{event: sampleEvent()},
		Publisher:  &fakePublisher{},
	}

	if _, err := runtime.RunOnce(context.Background()); err == nil {
		t.Fatal("expected fetch error")
	}
	if runtime.started || source.stopCount != 1 {
		t.Fatalf("expected source reset, started=%t stop_count=%d", runtime.started, source.stopCount)
	}

	source.fetchErr = nil
	source.changes = []cdc.ChangeEvent{sampleChange()}
	source.offset = cdc.Offset{ReaderName: "edge-001", BatchID: 20, BinlogFile: "mysql-bin.000005"}
	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce after reset returned error: %v", err)
	}
	if result.Action != "published" || source.startCount != 2 || !source.committed {
		t.Fatalf("unexpected result=%+v source=%+v", result, source)
	}
}

func TestCanalUploadRuntimeReconnectsAfterCommitError(t *testing.T) {
	source := &fakeCanalBatchSource{
		changes:   []cdc.ChangeEvent{sampleChange()},
		offset:    cdc.Offset{ReaderName: "edge-001", BatchID: 21, BinlogFile: "mysql-bin.000006"},
		commitErr: errors.New("ack failed"),
	}
	runtime := &CanalUploadRuntime{
		Source:     source,
		Decider:    fakeDecider{decision: loop.Decision{Upload: true}},
		Normalizer: fakeNormalizer{event: sampleEvent()},
		Publisher:  &fakePublisher{},
	}

	if _, err := runtime.RunOnce(context.Background()); err == nil {
		t.Fatal("expected commit error")
	}
	if runtime.started || source.stopCount != 1 {
		t.Fatalf("expected source reset after commit error, started=%t stop_count=%d", runtime.started, source.stopCount)
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

func TestServerCanalDispatchRuntimeDispatchesThenCommits(t *testing.T) {
	source := &fakeCanalBatchSource{
		changes: []cdc.ChangeEvent{sampleServerChange()},
		offset:  cdc.Offset{ReaderName: "server-001", BatchID: 11, BinlogFile: "mysql-bin.000002"},
	}
	dispatcher := &fakeDispatcher{}
	result, err := (&ServerCanalDispatchRuntime{
		Source:     source,
		Decider:    fakeDecider{decision: loop.Decision{Upload: true}},
		Normalizer: fakeNormalizer{event: sampleServerEvent()},
		Rules:      sampleServerRules(),
		Dispatcher: dispatcher,
		EdgeNodes:  []string{"edge-001", "edge-002"},
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "dispatched" || result.EventID != "evt-server-001" || result.DispatchCount != 2 {
		t.Fatalf("unexpected result %+v", result)
	}
	if !source.started || !source.committed {
		t.Fatalf("expected start and commit, got %+v", source)
	}
	if len(dispatcher.targets) != 2 {
		t.Fatalf("expected two dispatches, got %+v", dispatcher.targets)
	}
}

func TestServerCanalDispatchRuntimeSuppressesThenCommits(t *testing.T) {
	source := &fakeCanalBatchSource{
		changes: []cdc.ChangeEvent{sampleServerChange()},
		offset:  cdc.Offset{ReaderName: "server-001", BatchID: 11, BinlogFile: "mysql-bin.000002"},
	}
	dispatcher := &fakeDispatcher{}
	result, err := (&ServerCanalDispatchRuntime{
		Source:     source,
		Decider:    fakeDecider{decision: loop.Decision{Upload: false}},
		Normalizer: fakeNormalizer{event: sampleServerEvent()},
		Rules:      sampleServerRules(),
		Dispatcher: dispatcher,
		EdgeNodes:  []string{"edge-001"},
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "suppressed" || result.DispatchCount != 0 || !source.committed {
		t.Fatalf("unexpected result=%+v committed=%t", result, source.committed)
	}
	if len(dispatcher.targets) != 0 {
		t.Fatalf("suppressed server cdc should not dispatch, got %+v", dispatcher.targets)
	}
}

func TestServerCanalDispatchRuntimeCommitsEmptyCanalBatch(t *testing.T) {
	source := &fakeCanalBatchSource{offset: cdc.Offset{ReaderName: "server-001", BatchID: 13, BinlogFile: "mysql-bin.000004"}}
	result, err := (&ServerCanalDispatchRuntime{
		Source: source,
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "committed-empty" || !source.committed {
		t.Fatalf("unexpected result=%+v committed=%t", result, source.committed)
	}
}

func TestServerCanalDispatchRuntimeReconnectsAfterFetchError(t *testing.T) {
	source := &fakeCanalBatchSource{fetchErr: errors.New("canal connection closed")}
	runtime := &ServerCanalDispatchRuntime{Source: source}

	if _, err := runtime.RunOnce(context.Background()); err == nil {
		t.Fatal("expected fetch error")
	}
	if runtime.started || source.stopCount != 1 {
		t.Fatalf("expected source reset, started=%t stop_count=%d", runtime.started, source.stopCount)
	}
}

type fakeCanalBatchSource struct {
	changes    []cdc.ChangeEvent
	offset     cdc.Offset
	err        error
	startErr   error
	fetchErr   error
	commitErr  error
	started    bool
	stopped    bool
	startCount int
	stopCount  int
	committed  bool
}

func (s *fakeCanalBatchSource) Start(ctx context.Context) error {
	s.started = true
	s.startCount++
	if s.startErr != nil {
		return s.startErr
	}
	return s.err
}

func (s *fakeCanalBatchSource) Stop(ctx context.Context) error {
	s.stopped = true
	s.stopCount++
	return s.err
}

func (s *fakeCanalBatchSource) FetchChangesOnce(ctx context.Context) ([]cdc.ChangeEvent, cdc.Offset, error) {
	if s.fetchErr != nil {
		return nil, cdc.Offset{}, s.fetchErr
	}
	return s.changes, s.offset, s.err
}

func (s *fakeCanalBatchSource) Commit(ctx context.Context, offset cdc.Offset) error {
	s.committed = true
	if s.commitErr != nil {
		return s.commitErr
	}
	return s.err
}
