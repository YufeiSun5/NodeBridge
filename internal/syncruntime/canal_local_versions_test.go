package syncruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
)

type versionRecorderFunc func(context.Context, event.SyncEvent) error

func (f versionRecorderFunc) RecordLocal(ctx context.Context, evt event.SyncEvent) error {
	return f(ctx, evt)
}

func TestCanalLocalVersionRegistrationPrecedesPublishAndCommit(t *testing.T) {
	for _, server := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			name := map[bool]string{false: "edge", true: "server"}[server] + map[bool]string{false: "_success", true: "_failure"}[fail]
			t.Run(name, func(t *testing.T) {
				change, evt := sampleChange(), sampleEvent()
				if server {
					change, evt = sampleServerChange(), sampleServerEvent()
				}
				source := &fakeCanalBatchSource{changes: []cdc.ChangeEvent{change}, offset: cdc.Offset{BatchID: 10}}
				pub := &fakePublisher{}
				dispatcher := &fakeDispatcher{}
				called := 0
				failure := errors.New("version store unavailable")
				recorder := versionRecorderFunc(func(_ context.Context, actual event.SyncEvent) error {
					called++
					if source.committed || len(pub.requests) != 0 || len(dispatcher.targets) != 0 || actual.EventID != evt.EventID {
						t.Fatal("version registration was late or received wrong event")
					}
					if fail {
						return failure
					}
					return nil
				})
				var err error
				if server {
					_, err = (&ServerCanalDispatchRuntime{Source: source, Normalizer: fakeNormalizer{event: evt}, Rules: sampleServerRules(), Dispatcher: dispatcher, EdgeNodes: []string{"edge-001"}, LocalVersions: recorder}).RunOnce(context.Background())
				} else {
					_, err = (&CanalUploadRuntime{Source: source, Normalizer: fakeNormalizer{event: evt}, Publisher: pub, LocalVersions: recorder}).RunOnce(context.Background())
				}
				if called != 1 || (fail && !errors.Is(err, failure)) || (!fail && err != nil) {
					t.Fatalf("calls=%d error=%v", called, err)
				}
				if fail {
					if source.committed || len(pub.requests) != 0 || len(dispatcher.targets) != 0 || !source.stopped {
						t.Fatal("failed registration did not block and reset capture")
					}
				} else if !source.committed {
					t.Fatal("successful registration was not followed by commit")
				}
			})
		}
	}
}

func TestCanalReplayDoesNotRegisterLocalVersion(t *testing.T) {
	for _, server := range []bool{false, true} {
		source := &fakeCanalBatchSource{changes: []cdc.ChangeEvent{sampleChange()}, offset: cdc.Offset{BatchID: 1}}
		recorder := versionRecorderFunc(func(context.Context, event.SyncEvent) error { t.Fatal("replay registered as local write"); return nil })
		decider := fakeDecider{decision: loop.Decision{Upload: false}}
		var err error
		if server {
			_, err = (&ServerCanalDispatchRuntime{Source: source, Decider: decider, Normalizer: fakeNormalizer{}, Rules: sampleServerRules(), Dispatcher: &fakeDispatcher{}, LocalVersions: recorder}).RunOnce(context.Background())
		} else {
			_, err = (&CanalUploadRuntime{Source: source, Decider: decider, Normalizer: fakeNormalizer{}, Publisher: &fakePublisher{}, LocalVersions: recorder}).RunOnce(context.Background())
		}
		if err != nil || !source.committed {
			t.Fatalf("unexpected %v", err)
		}
	}
}
