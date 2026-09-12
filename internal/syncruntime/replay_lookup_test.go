package syncruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
)

func TestCanalReplayLookupFailureDoesNotPublishOrAcknowledge(t *testing.T) {
	for _, server := range []bool{false, true} {
		t.Run(map[bool]string{false: "upload", true: "dispatch"}[server], func(t *testing.T) {
			failure := errors.New("replay query failed")
			source := &fakeCanalBatchSource{changes: []cdc.ChangeEvent{sampleServerChange()}, offset: cdc.Offset{BatchID: 1}}
			publisher := &fakePublisher{}
			dispatcher := &fakeDispatcher{}
			upload := &CanalUploadRuntime{Source: source, Decider: fakeDecider{err: failure}, Normalizer: fakeNormalizer{event: sampleEvent()}, Publisher: publisher}
			dispatch := &ServerCanalDispatchRuntime{Source: source, Decider: fakeDecider{err: failure}, Normalizer: fakeNormalizer{event: sampleServerEvent()}, Rules: sampleServerRules(), Dispatcher: dispatcher, EdgeNodes: []string{"edge-001"}}
			run := upload.RunOnce
			if server {
				run = dispatch.RunOnce
			}
			if _, err := run(context.Background()); !errors.Is(err, failure) {
				t.Fatalf("lookup error not returned: %v", err)
			}
			if source.committed || source.stopCount != 1 || len(publisher.requests) != 0 || len(dispatcher.targets) != 0 {
				t.Fatalf("lookup failure acknowledged or published: source=%+v publish=%d dispatch=%d", source, len(publisher.requests), len(dispatcher.targets))
			}
			upload.Decider, dispatch.Decider = fakeDecider{}, fakeDecider{}
			if result, err := run(context.Background()); err != nil || result.Action != "suppressed" || !source.committed || source.startCount != 2 {
				t.Fatalf("unacknowledged batch not retried: result=%+v err=%v source=%+v", result, err, source)
			}
		})
	}
}

type laterReplayFailure struct {
	calls int
	err   error
}

func (d *laterReplayFailure) ShouldUpload(_ context.Context, _ cdc.ChangeEvent) (loop.Decision, error) {
	d.calls++
	if d.calls == 2 {
		return loop.Decision{}, d.err
	}
	return loop.Decision{Upload: true}, nil
}

func TestLaterReplayLookupFailureRetainsEntireCanalBatch(t *testing.T) {
	for _, server := range []bool{false, true} {
		failure := errors.New("second replay query failed")
		source := &fakeCanalBatchSource{changes: []cdc.ChangeEvent{sampleServerChange(), sampleServerChange()}, offset: cdc.Offset{BatchID: 1}}
		decider := &laterReplayFailure{err: failure}
		publisher := &fakePublisher{}
		dispatcher := &fakeDispatcher{}
		upload := &CanalUploadRuntime{Source: source, Decider: decider, Normalizer: fakeNormalizer{event: sampleEvent()}, Publisher: publisher}
		dispatch := &ServerCanalDispatchRuntime{Source: source, Decider: decider, Normalizer: fakeNormalizer{event: sampleServerEvent()}, Rules: sampleServerRules(), Dispatcher: dispatcher, EdgeNodes: []string{"edge-001"}}
		run := upload.RunOnce
		if server {
			run = dispatch.RunOnce
		}
		if _, err := run(context.Background()); !errors.Is(err, failure) {
			t.Fatalf("server=%t lost later replay error: %v", server, err)
		}
		if source.committed || source.stopCount != 1 || len(publisher.requests) != 0 || len(dispatcher.targets) != 0 {
			t.Fatalf("server=%t partially published or acknowledged unverified batch", server)
		}
	}
}

func TestCDCReplayLookupFailureDoesNotPublish(t *testing.T) {
	failure := errors.New("replay query failed")
	source := &fakeChangeSource{change: sampleServerChange(), ok: true}
	publisher := &fakePublisher{}
	dispatcher := &fakeDispatcher{}
	upload := CDCUploadRuntime{Source: source, Decider: fakeDecider{err: failure}, Normalizer: fakeNormalizer{event: sampleEvent()}, Publisher: publisher}
	dispatch := ServerCDCDispatchRuntime{Source: source, Decider: fakeDecider{err: failure}, Normalizer: fakeNormalizer{event: sampleServerEvent()}, Rules: sampleServerRules(), Dispatcher: dispatcher, EdgeNodes: []string{"edge-001"}}
	if _, err := upload.RunOnce(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("upload error lost: %v", err)
	}
	if _, err := dispatch.RunOnce(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("dispatch error lost: %v", err)
	}
	if len(publisher.requests) != 0 || len(dispatcher.targets) != 0 {
		t.Fatal("replay lookup error published a CDC event")
	}
}
