package syncruntime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
)

func TestCDCUploadRuntimePublishesNormalizedEvent(t *testing.T) {
	publisher := &fakePublisher{}
	runtime := CDCUploadRuntime{
		Source:     &fakeChangeSource{change: sampleChange(), ok: true},
		Decider:    fakeDecider{decision: loop.Decision{Upload: true, Reason: "local"}},
		Normalizer: fakeNormalizer{event: sampleEvent()},
		Publisher:  publisher,
		Exchange:   "edge.upload.x",
		RoutingKey: "edge.upload.cdc",
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if result.Action != "published" || result.EventID != "evt-001" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(publisher.requests) != 1 {
		t.Fatalf("expected one publish, got %d", len(publisher.requests))
	}
	req := publisher.requests[0]
	if req.Exchange != "edge.upload.x" || req.RoutingKey != "edge.upload.cdc" {
		t.Fatalf("unexpected publish request %+v", req)
	}
	var evt event.SyncEvent
	if err := json.Unmarshal(req.Body, &evt); err != nil {
		t.Fatalf("parse published body: %v", err)
	}
	if evt.EventID != "evt-001" {
		t.Fatalf("unexpected event %+v", evt)
	}
}

func TestCDCUploadRuntimeSuppressesReplay(t *testing.T) {
	publisher := &fakePublisher{}
	result, err := (CDCUploadRuntime{
		Source:     &fakeChangeSource{change: sampleChange(), ok: true},
		Decider:    fakeDecider{decision: loop.Decision{Upload: false, Reason: "replayed"}},
		Normalizer: fakeNormalizer{event: sampleEvent()},
		Publisher:  publisher,
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "suppressed" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(publisher.requests) != 0 {
		t.Fatalf("suppressed change should not publish, got %d", len(publisher.requests))
	}
}

func TestCDCUploadRuntimeReturnsEmpty(t *testing.T) {
	result, err := (CDCUploadRuntime{
		Source:     &fakeChangeSource{},
		Normalizer: fakeNormalizer{event: sampleEvent()},
		Publisher:  &fakePublisher{},
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Processed || result.Action != "empty" {
		t.Fatalf("unexpected result %+v", result)
	}
}

func TestCDCUploadRuntimePropagatesErrors(t *testing.T) {
	_, err := (CDCUploadRuntime{
		Source:     &fakeChangeSource{change: sampleChange(), ok: true},
		Normalizer: fakeNormalizer{err: errors.New("bad change")},
		Publisher:  &fakePublisher{},
	}).RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected normalizer error")
	}

	_, err = (CDCUploadRuntime{
		Source:     &fakeChangeSource{change: sampleChange(), ok: true},
		Normalizer: fakeNormalizer{event: sampleEvent()},
		Publisher:  &fakePublisher{err: errors.New("broker down")},
	}).RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected publish error")
	}
}

type fakeChangeSource struct {
	change cdc.ChangeEvent
	ok     bool
	err    error
}

func (s *fakeChangeSource) GetChange(ctx context.Context) (cdc.ChangeEvent, bool, error) {
	return s.change, s.ok, s.err
}

type fakeDecider struct {
	decision loop.Decision
}

func (d fakeDecider) ShouldUpload(change cdc.ChangeEvent) loop.Decision {
	return d.decision
}

type fakeNormalizer struct {
	event event.SyncEvent
	err   error
}

func (n fakeNormalizer) Normalize(change cdc.ChangeEvent) (event.SyncEvent, error) {
	return n.event, n.err
}

func sampleChange() cdc.ChangeEvent {
	return cdc.ChangeEvent{
		DatabaseName: "scada_edge",
		TableName:    "device_config",
		Operation:    cdc.OperationUpdate,
		PrimaryKey:   map[string]any{"id": 1},
		After:        map[string]any{"id": 1, "name": "Pump A"},
	}
}

var _ = rabbitmq.PublishRequest{}
