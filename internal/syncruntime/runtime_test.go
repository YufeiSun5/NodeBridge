package syncruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestEdgeUploadRuntimeForwardsAndAcks(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	publisher := &fakePublisher{}
	runtime := EdgeUploadRuntime{
		Source:     &fakeSource{msg: msg, ok: true},
		Publisher:  publisher,
		Exchange:   "server.ingress.x",
		RoutingKey: "server.ingress",
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if !result.Processed || result.EventID != "evt-001" || result.Action != "forwarded" {
		t.Fatalf("unexpected result %+v", result)
	}
	if !msg.acked || msg.nacked {
		t.Fatalf("expected ack only, got ack=%t nack=%t", msg.acked, msg.nacked)
	}
	if len(publisher.requests) != 1 {
		t.Fatalf("expected one publish, got %d", len(publisher.requests))
	}
	if publisher.requests[0].Exchange != "server.ingress.x" || publisher.requests[0].RoutingKey != "server.ingress" {
		t.Fatalf("unexpected publish request %+v", publisher.requests[0])
	}
}

func TestEdgeUploadRuntimeNacksOnPublishFailure(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	runtime := EdgeUploadRuntime{
		Source:    &fakeSource{msg: msg, ok: true},
		Publisher: &fakePublisher{err: errors.New("broker down")},
		Consumer:  rabbitmq.Consumer{RequeueOnError: true},
	}

	result, err := runtime.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected publish failure")
	}
	if result.Action != "failed" || result.EventID != "evt-001" {
		t.Fatalf("unexpected result %+v", result)
	}
	if msg.acked || !msg.nacked || !msg.requeue {
		t.Fatalf("expected requeue nack, got ack=%t nack=%t requeue=%t", msg.acked, msg.nacked, msg.requeue)
	}
}

func TestServerIngressRuntimeAppliesDispatchesAndAcks(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	worker := &fakeWorker{}
	dispatcher := &fakeDispatcher{}
	runtime := ServerIngressRuntime{
		Source:     &fakeSource{msg: msg, ok: true},
		Rules:      sampleRules(),
		Worker:     worker,
		Dispatcher: dispatcher,
		EdgeNodes:  []string{"edge-a", "edge-b", "edge-c"},
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if result.Action != "applied" || result.EventID != "evt-001" || result.DispatchCount != 2 {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(worker.events) != 1 {
		t.Fatalf("expected one apply, got %d", len(worker.events))
	}
	if worker.events[0].TargetTable != "device_settings" {
		t.Fatalf("expected mapped target table, got %s", worker.events[0].TargetTable)
	}
	if len(dispatcher.targets) != 2 {
		t.Fatalf("expected dispatch to two non-origin nodes, got %+v", dispatcher.targets)
	}
	for _, target := range dispatcher.targets {
		if target == "edge-a" {
			t.Fatal("server must not dispatch back to origin")
		}
	}
	if !msg.acked || msg.nacked {
		t.Fatalf("expected ack only, got ack=%t nack=%t", msg.acked, msg.nacked)
	}
}

func TestServerIngressRuntimeDisabledRuleAcksWithoutApply(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	set := sampleRules()
	set.Rules[0].Enable = false
	worker := &fakeWorker{}
	runtime := ServerIngressRuntime{
		Source: &fakeSource{msg: msg, ok: true},
		Rules:  set,
		Worker: worker,
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "applied" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(worker.events) != 0 {
		t.Fatalf("disabled rule should not apply, got %d", len(worker.events))
	}
	if !msg.acked {
		t.Fatal("disabled rule should still ack the message")
	}
}

func TestServerIngressRuntimeNacksOnApplyFailure(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	runtime := ServerIngressRuntime{
		Source:   &fakeSource{msg: msg, ok: true},
		Consumer: rabbitmq.Consumer{RequeueOnError: true},
		Rules:    sampleRules(),
		Worker:   &fakeWorker{err: errors.New("mysql down")},
	}

	result, err := runtime.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected apply failure")
	}
	if result.Action != "failed" {
		t.Fatalf("unexpected result %+v", result)
	}
	if msg.acked || !msg.nacked || !msg.requeue {
		t.Fatalf("expected requeue nack, got ack=%t nack=%t requeue=%t", msg.acked, msg.nacked, msg.requeue)
	}
}

func TestEdgeDownlinkRuntimeAppliesAndAcks(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	worker := &fakeWorker{}
	runtime := EdgeDownlinkRuntime{
		Source: &fakeSource{msg: msg, ok: true},
		Rules:  sampleRules(),
		Worker: worker,
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if result.Action != "applied" || result.EventID != "evt-001" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(worker.events) != 1 {
		t.Fatalf("expected one apply, got %d", len(worker.events))
	}
	if !msg.acked || msg.nacked {
		t.Fatalf("expected ack only, got ack=%t nack=%t", msg.acked, msg.nacked)
	}
}

func TestEdgeDownlinkRuntimeNacksOnApplyFailure(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	runtime := EdgeDownlinkRuntime{
		Source:   &fakeSource{msg: msg, ok: true},
		Consumer: rabbitmq.Consumer{RequeueOnError: true},
		Rules:    sampleRules(),
		Worker:   &fakeWorker{err: errors.New("edge mysql down")},
	}

	result, err := runtime.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected apply failure")
	}
	if result.Action != "failed" {
		t.Fatalf("unexpected result %+v", result)
	}
	if msg.acked || !msg.nacked || !msg.requeue {
		t.Fatalf("expected requeue nack, got ack=%t nack=%t requeue=%t", msg.acked, msg.nacked, msg.requeue)
	}
}

func TestEdgeDownlinkRuntimeDisabledRuleAcksWithoutApply(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	set := sampleRules()
	set.Rules[0].Enable = false
	worker := &fakeWorker{}
	runtime := EdgeDownlinkRuntime{
		Source: &fakeSource{msg: msg, ok: true},
		Rules:  set,
		Worker: worker,
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "applied" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(worker.events) != 0 {
		t.Fatalf("disabled rule should not apply, got %d", len(worker.events))
	}
	if !msg.acked {
		t.Fatal("disabled rule should still ack the message")
	}
}

func TestRuntimeEmptySource(t *testing.T) {
	result, err := EdgeUploadRuntime{
		Source:    &fakeSource{},
		Publisher: &fakePublisher{},
	}.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Processed || result.Action != "empty" {
		t.Fatalf("unexpected result %+v", result)
	}
}

type fakeSource struct {
	msg rabbitmq.IncomingMessage
	ok  bool
	err error
}

func (s *fakeSource) Get(ctx context.Context) (rabbitmq.IncomingMessage, bool, error) {
	return s.msg, s.ok, s.err
}

type fakeMessage struct {
	body    []byte
	acked   bool
	nacked  bool
	requeue bool
}

func (m *fakeMessage) Body() []byte {
	return m.body
}

func (m *fakeMessage) Ack(multiple bool) error {
	m.acked = true
	return nil
}

func (m *fakeMessage) Nack(multiple, requeue bool) error {
	m.nacked = true
	m.requeue = requeue
	return nil
}

type fakePublisher struct {
	requests []rabbitmq.PublishRequest
	err      error
}

func (p *fakePublisher) Publish(ctx context.Context, req rabbitmq.PublishRequest) error {
	p.requests = append(p.requests, req)
	return p.err
}

type fakeWorker struct {
	events []mapper.MappedEvent
	err    error
}

func (w *fakeWorker) Apply(ctx context.Context, evt mapper.MappedEvent) (apply.Result, error) {
	w.events = append(w.events, evt)
	return apply.Result{EventID: evt.Event.EventID, SourceTable: evt.SourceTable, TargetTable: evt.TargetTable}, w.err
}

type fakeDispatcher struct {
	targets []string
	err     error
}

func (d *fakeDispatcher) Dispatch(ctx context.Context, evt event.SyncEvent, targetNodeID string) error {
	d.targets = append(d.targets, targetNodeID)
	return d.err
}

func sampleEvent() event.SyncEvent {
	return event.SyncEvent{
		EventID:      "evt-001",
		EventType:    "UPDATE",
		OriginNodeID: "edge-a",
		SourceNodeID: "edge-a",
		DatabaseName: "scada_edge",
		TableName:    "device_config",
		PrimaryKey:   map[string]any{"id": 1},
		After: map[string]any{
			"id":    1,
			"name":  "Pump A",
			"value": "ON",
		},
		SchemaVersion: 1,
		CreatedAt:     time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
		EventTime:     time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
		TraceID:       "trace-001",
	}
}

func sampleRules() *rules.RuleSet {
	return &rules.RuleSet{Rules: []rules.SyncRule{
		{
			ID:                 "device-config",
			DatabaseName:       "scada_edge",
			TableName:          "device_config",
			TargetDatabaseName: "scada_center",
			TargetTableName:    "device_settings",
			Direction:          rules.DirectionBidirectional,
			ConflictPolicy:     rules.ConflictLastWriteWin,
			Enable:             true,
			PrimaryKeys:        []string{"id"},
			TargetPrimaryKeys:  []string{"setting_id"},
			ColumnMappings: []rules.ColumnMapping{
				{SourceColumn: "id", TargetColumn: "setting_id"},
				{SourceColumn: "name", TargetColumn: "display_name"},
				{SourceColumn: "value", TargetColumn: "setting_value"},
			},
		},
	}}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := rabbitmq.EncodeJSON(value)
	if err != nil {
		t.Fatalf("encode json: %v", err)
	}
	return body
}
