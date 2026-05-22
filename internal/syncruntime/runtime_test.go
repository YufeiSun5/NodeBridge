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
	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
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

func TestEdgeUploadBatchRuntimeFlushesAtMaxBatchInOrder(t *testing.T) {
	messages := []*fakeMessage{
		{body: mustJSON(t, sampleEventWithID("evt-001", 1))},
		{body: mustJSON(t, sampleEventWithID("evt-002", 2))},
		{body: mustJSON(t, sampleEventWithID("evt-003", 3))},
	}
	publisher := &fakePublisher{}
	result, err := (EdgeUploadBatchRuntime{
		Source:        &fakeBatchSource{messages: incomingRuntime(messages)},
		Publisher:     publisher,
		Consumer:      rabbitmq.Consumer{RequeueOnError: true},
		Exchange:      "server.ingress.x",
		RoutingKey:    "server.ingress",
		MaxBatch:      3,
		FlushInterval: time.Hour,
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Count != 3 || result.EventID != "evt-003" || result.Action != "forwarded" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(publisher.requests) != 3 {
		t.Fatalf("expected three publishes, got %d", len(publisher.requests))
	}
	for i, msg := range messages {
		if !msg.acked || msg.nacked {
			t.Fatalf("message %d expected ack only, got %+v", i, msg)
		}
	}
}

func TestEdgeUploadBatchRuntimeNacksFailureAndRest(t *testing.T) {
	messages := []*fakeMessage{
		{body: mustJSON(t, sampleEventWithID("evt-001", 1))},
		{body: mustJSON(t, sampleEventWithID("evt-002", 2))},
		{body: mustJSON(t, sampleEventWithID("evt-003", 3))},
	}
	result, err := (EdgeUploadBatchRuntime{
		Source:        &fakeBatchSource{messages: incomingRuntime(messages)},
		Publisher:     &fakePublisher{failOnCall: 2, err: errors.New("broker down")},
		Consumer:      rabbitmq.Consumer{RequeueOnError: true},
		MaxBatch:      3,
		FlushInterval: time.Hour,
	}).RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected batch publish failure")
	}
	if result.EventID != "evt-002" || result.Action != "failed" {
		t.Fatalf("unexpected result %+v", result)
	}
	if !messages[0].acked || messages[0].nacked {
		t.Fatalf("first message should be acked, got %+v", messages[0])
	}
	for i := 1; i < len(messages); i++ {
		if messages[i].acked || !messages[i].nacked || !messages[i].requeue {
			t.Fatalf("message %d should be requeue nacked, got %+v", i, messages[i])
		}
	}
}

func TestServerIngressRuntimeAppliesDispatchesAndAcks(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	worker := &fakeWorker{}
	dispatcher := &fakeDispatcher{}
	eventStore := &fakeEventStore{}
	runtime := ServerIngressRuntime{
		Source:     &fakeSource{msg: msg, ok: true},
		Rules:      sampleRules(),
		Worker:     worker,
		EventStore: eventStore,
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
	if len(eventStore.records) != 2 || eventStore.records[0].Status != syncstore.StatusPending || eventStore.records[1].Status != syncstore.StatusSuccess {
		t.Fatalf("expected pending and success event logs, got %+v", eventStore.records)
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

func TestServerIngressRuntimeUsesActiveNodeStore(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	dispatcher := &fakeDispatcher{}
	runtime := ServerIngressRuntime{
		Source:     &fakeSource{msg: msg, ok: true},
		Rules:      sampleRules(),
		Worker:     &fakeWorker{},
		Dispatcher: dispatcher,
		NodeStore:  &fakeActiveNodeStore{nodes: []string{"edge-a", "edge-b"}},
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.DispatchCount != 1 {
		t.Fatalf("expected one non-origin dispatch, got %+v", result)
	}
	if len(dispatcher.targets) != 1 || dispatcher.targets[0] != "edge-b" {
		t.Fatalf("unexpected dynamic dispatch targets %+v", dispatcher.targets)
	}
}

func TestServerIngressRuntimeEdgeToServerDoesNotDispatch(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	dispatcher := &fakeDispatcher{}
	set := sampleRules()
	set.Rules[0].Direction = rules.DirectionEdgeToServer
	runtime := ServerIngressRuntime{
		Source:     &fakeSource{msg: msg, ok: true},
		Rules:      set,
		Worker:     &fakeWorker{},
		Dispatcher: dispatcher,
		EdgeNodes:  []string{"edge-a", "edge-b"},
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.DispatchCount != 0 {
		t.Fatalf("EDGE_TO_SERVER must not dispatch, got %+v", result)
	}
	if len(dispatcher.targets) != 0 {
		t.Fatalf("unexpected dispatch targets %+v", dispatcher.targets)
	}
}

func TestServerIngressRuntimeEdgeToServerCanDispatchWhenConfigured(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	dispatcher := &fakeDispatcher{}
	set := sampleRules()
	set.Rules[0].Direction = rules.DirectionEdgeToServer
	set.Rules[0].DispatchTarget = rules.DispatchActiveEdges
	runtime := ServerIngressRuntime{
		Source:     &fakeSource{msg: msg, ok: true},
		Rules:      set,
		Worker:     &fakeWorker{},
		Dispatcher: dispatcher,
		EdgeNodes:  []string{"edge-a", "edge-b", "edge-c"},
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.DispatchCount != 2 {
		t.Fatalf("expected configured dispatch, got %+v", result)
	}
	if len(dispatcher.targets) != 2 || dispatcher.targets[0] != "edge-b" || dispatcher.targets[1] != "edge-c" {
		t.Fatalf("unexpected dispatch targets %+v", dispatcher.targets)
	}
}

func TestServerIngressRuntimeBidirectionalCanDisableDispatch(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	dispatcher := &fakeDispatcher{}
	set := sampleRules()
	set.Rules[0].DispatchTarget = rules.DispatchNone
	runtime := ServerIngressRuntime{
		Source:     &fakeSource{msg: msg, ok: true},
		Rules:      set,
		Worker:     &fakeWorker{},
		Dispatcher: dispatcher,
		EdgeNodes:  []string{"edge-a", "edge-b"},
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.DispatchCount != 0 || len(dispatcher.targets) != 0 {
		t.Fatalf("expected dispatch disabled, result=%+v targets=%+v", result, dispatcher.targets)
	}
}

func TestServerIngressRuntimeDispatchesSelectedNodes(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	dispatcher := &fakeDispatcher{}
	set := sampleRules()
	set.Rules[0].DispatchTarget = rules.DispatchSelectedEdges
	set.Rules[0].DispatchNodeIDs = []string{"edge-a", "edge-c"}
	runtime := ServerIngressRuntime{
		Source:     &fakeSource{msg: msg, ok: true},
		Rules:      set,
		Worker:     &fakeWorker{},
		Dispatcher: dispatcher,
		EdgeNodes:  []string{"edge-a", "edge-b", "edge-c"},
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.DispatchCount != 1 {
		t.Fatalf("expected one selected non-origin dispatch, got %+v", result)
	}
	if len(dispatcher.targets) != 1 || dispatcher.targets[0] != "edge-c" {
		t.Fatalf("unexpected selected targets %+v", dispatcher.targets)
	}
}

func TestServerIngressRuntimeUsesNodeScopedRule(t *testing.T) {
	evt := sampleEvent()
	evt.TableName = "data_all"
	evt.OriginNodeID = "edge-b"
	evt.SourceNodeID = "edge-b"
	evt.After = map[string]any{"id": 1, "value": "B"}
	msg := &fakeMessage{body: mustJSON(t, evt)}
	worker := &fakeWorker{}
	set := &rules.RuleSet{Rules: []rules.SyncRule{
		{
			ID:                 "data-all-edge-a",
			DatabaseName:       "scada_edge",
			TableName:          "data_all",
			SourceNodeIDs:      []string{"edge-a"},
			TargetDatabaseName: "scada_center",
			TargetTableName:    "data_all_edge_a",
			Direction:          rules.DirectionEdgeToServer,
			Enable:             true,
			PrimaryKeys:        []string{"id"},
		},
		{
			ID:                 "data-all-edge-b",
			DatabaseName:       "scada_edge",
			TableName:          "data_all",
			SourceNodeIDs:      []string{"edge-b"},
			TargetDatabaseName: "scada_center",
			TargetTableName:    "data_all_edge_b",
			Direction:          rules.DirectionEdgeToServer,
			Enable:             true,
			PrimaryKeys:        []string{"id"},
		},
	}}
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
	if len(worker.events) != 1 || worker.events[0].TargetTable != "data_all_edge_b" {
		t.Fatalf("expected edge-b target table, got %+v", worker.events)
	}
}

func TestServerIngressBatchRuntimeEdgeToServerDoesNotDispatch(t *testing.T) {
	messages := []*fakeMessage{
		{body: mustJSON(t, sampleEventWithID("evt-001", 1))},
		{body: mustJSON(t, sampleEventWithID("evt-002", 2))},
	}
	dispatcher := &fakeDispatcher{}
	set := sampleRules()
	set.Rules[0].Direction = rules.DirectionEdgeToServer
	result, err := (ServerIngressBatchRuntime{
		Source:        &fakeBatchSource{messages: incomingRuntime(messages)},
		Rules:         set,
		Worker:        &fakeWorker{},
		Dispatcher:    dispatcher,
		EdgeNodes:     []string{"edge-a", "edge-b"},
		MaxBatch:      2,
		FlushInterval: time.Hour,
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Count != 2 || result.DispatchCount != 0 {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(dispatcher.targets) != 0 {
		t.Fatalf("unexpected dispatch targets %+v", dispatcher.targets)
	}
}

func TestServerIngressRuntimeNacksOnEventStoreFailure(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	runtime := ServerIngressRuntime{
		Source:     &fakeSource{msg: msg, ok: true},
		Consumer:   rabbitmq.Consumer{RequeueOnError: true},
		Rules:      sampleRules(),
		Worker:     &fakeWorker{},
		EventStore: &fakeEventStore{err: errors.New("mysql log down")},
	}

	result, err := runtime.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected event store failure")
	}
	if result.Action != "failed" || msg.acked || !msg.nacked || !msg.requeue {
		t.Fatalf("unexpected result=%+v ack=%t nack=%t requeue=%t", result, msg.acked, msg.nacked, msg.requeue)
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

func TestEdgeDownlinkBatchRuntimeAppliesInOrder(t *testing.T) {
	messages := []*fakeMessage{
		{body: mustJSON(t, sampleEventWithID("evt-001", 1))},
		{body: mustJSON(t, sampleEventWithID("evt-002", 2))},
	}
	worker := &fakeWorker{}
	result, err := (EdgeDownlinkBatchRuntime{
		Source:        &fakeBatchSource{messages: incomingRuntime(messages)},
		Rules:         sampleRules(),
		Worker:        worker,
		MaxBatch:      2,
		FlushInterval: time.Hour,
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Count != 2 || result.EventID != "evt-002" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(worker.events) != 2 || worker.events[0].Event.EventID != "evt-001" || worker.events[1].Event.EventID != "evt-002" {
		t.Fatalf("unexpected apply order %+v", worker.events)
	}
}

func TestEdgeDownlinkRuntimeOverridesTargetDatabase(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	worker := &fakeWorker{}
	runtime := EdgeDownlinkRuntime{
		Source:                 &fakeSource{msg: msg, ok: true},
		Rules:                  sampleRules(),
		Worker:                 worker,
		TargetDatabaseOverride: "scada_edge",
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "applied" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(worker.events) != 1 {
		t.Fatalf("expected one apply, got %d", len(worker.events))
	}
	if worker.events[0].TargetDatabase != "scada_edge" {
		t.Fatalf("expected local target database, got %s", worker.events[0].TargetDatabase)
	}
}

func TestEdgeDownlinkRuntimeAppliesConfigUpdateAndAcks(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleConfigEvent())}
	store := &fakeConfigStore{}
	runtime := EdgeDownlinkRuntime{
		Source:      &fakeSource{msg: msg, ok: true},
		Rules:       sampleRules(),
		Worker:      &fakeWorker{},
		ConfigStore: store,
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.EventID != "cfg-001" || result.Action != "applied" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(store.configs) != 1 || store.configs[0].NodeID != "edge-b" || store.configs[0].MySQLHost != "127.0.0.1" {
		t.Fatalf("unexpected config store %+v", store.configs)
	}
	if !msg.acked || msg.nacked {
		t.Fatalf("expected ack only, got ack=%t nack=%t", msg.acked, msg.nacked)
	}
}

func TestEdgeDownlinkRuntimeNacksConfigUpdateFailure(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleConfigEvent())}
	runtime := EdgeDownlinkRuntime{
		Source:      &fakeSource{msg: msg, ok: true},
		Consumer:    rabbitmq.Consumer{RequeueOnError: true},
		Rules:       sampleRules(),
		Worker:      &fakeWorker{},
		ConfigStore: &fakeConfigStore{err: errors.New("config db down")},
	}

	result, err := runtime.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected config apply failure")
	}
	if result.Action != "failed" || !msg.nacked || !msg.requeue || msg.acked {
		t.Fatalf("unexpected failure result=%+v ack=%t nack=%t requeue=%t", result, msg.acked, msg.nacked, msg.requeue)
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

func TestReplayRuntimeDispatchesPendingEvent(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	store := &fakeReplayStore{
		items: []syncstore.ReplayEvent{
			{
				EventID:      "evt-001",
				TargetNodeID: "edge-b",
				Payload:      mustJSON(t, sampleEvent()),
			},
		},
	}
	runtime := ReplayRuntime{Store: store, Dispatcher: dispatcher}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "replayed" || result.EventID != "evt-001" || result.DispatchCount != 1 {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(dispatcher.targets) != 1 || dispatcher.targets[0] != "edge-b" {
		t.Fatalf("unexpected targets %+v", dispatcher.targets)
	}
	if len(store.acks) != 1 || store.acks[0].Status != syncstore.StatusSuccess {
		t.Fatalf("expected success ack, got %+v", store.acks)
	}
	if len(store.dispatches) != 1 || store.dispatches[0].Status != syncstore.StatusSuccess {
		t.Fatalf("expected success dispatch, got %+v", store.dispatches)
	}
}

func TestReplayRuntimeMarksInvalidPayloadFailed(t *testing.T) {
	store := &fakeReplayStore{
		items: []syncstore.ReplayEvent{
			{EventID: "evt-001", TargetNodeID: "edge-b", Payload: []byte("{bad")},
		},
	}
	result, err := ReplayRuntime{Store: store, Dispatcher: &fakeDispatcher{}}.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected invalid payload error")
	}
	if result.Action != "failed" || len(store.acks) != 1 || store.acks[0].Status != syncstore.StatusFailed {
		t.Fatalf("unexpected result=%+v acks=%+v", result, store.acks)
	}
}

type fakeSource struct {
	msg rabbitmq.IncomingMessage
	ok  bool
	err error
}

type fakeBatchSource struct {
	messages []rabbitmq.IncomingMessage
}

func (s *fakeBatchSource) GetBatch(ctx context.Context, max int, flushInterval time.Duration) ([]rabbitmq.IncomingMessage, error) {
	if len(s.messages) <= max {
		return s.messages, nil
	}
	return s.messages[:max], nil
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
	requests   []rabbitmq.PublishRequest
	err        error
	failOnCall int
}

func (p *fakePublisher) Publish(ctx context.Context, req rabbitmq.PublishRequest) error {
	p.requests = append(p.requests, req)
	if p.failOnCall > 0 && len(p.requests) == p.failOnCall {
		return p.err
	}
	if p.failOnCall > 0 {
		return nil
	}
	return p.err
}

type fakeWorker struct {
	events []mapper.MappedEvent
	err    error
}

type fakeEventStore struct {
	records []syncstore.EventLogRecord
	err     error
}

func (s *fakeEventStore) UpsertEventLog(ctx context.Context, record syncstore.EventLogRecord) error {
	s.records = append(s.records, record)
	return s.err
}

type fakeReplayStore struct {
	items      []syncstore.ReplayEvent
	acks       []syncstore.AckRecord
	dispatches []syncstore.DispatchRecord
	err        error
}

type fakeActiveNodeStore struct {
	nodes []string
	err   error
}

func (s *fakeActiveNodeStore) ListActiveEdgeNodeIDs(ctx context.Context) ([]string, error) {
	return s.nodes, s.err
}

type fakeConfigStore struct {
	configs []syncstore.NodeConfig
	err     error
}

func (s *fakeConfigStore) UpsertNodeConfig(ctx context.Context, config syncstore.NodeConfig) error {
	s.configs = append(s.configs, config)
	return s.err
}

func (s *fakeReplayStore) ListPendingReplays(ctx context.Context, limit int) ([]syncstore.ReplayEvent, error) {
	return s.items, s.err
}

func (s *fakeReplayStore) UpsertAck(ctx context.Context, record syncstore.AckRecord) error {
	s.acks = append(s.acks, record)
	return s.err
}

func (s *fakeReplayStore) UpsertDispatch(ctx context.Context, record syncstore.DispatchRecord) error {
	s.dispatches = append(s.dispatches, record)
	return s.err
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
	return sampleEventWithID("evt-001", 1)
}

func sampleEventWithID(eventID string, id int) event.SyncEvent {
	return event.SyncEvent{
		EventID:      eventID,
		EventType:    "UPDATE",
		OriginNodeID: "edge-a",
		SourceNodeID: "edge-a",
		DatabaseName: "scada_edge",
		TableName:    "device_config",
		PrimaryKey:   map[string]any{"id": id},
		After: map[string]any{
			"id":    id,
			"name":  "Pump A",
			"value": "ON",
		},
		SchemaVersion: 1,
		CreatedAt:     time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
		EventTime:     time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
		TraceID:       "trace-001",
	}
}

func sampleConfigEvent() event.SyncEvent {
	return event.SyncEvent{
		EventID:      "cfg-001",
		EventType:    event.TypeConfigUpdate,
		OriginNodeID: "server-001",
		SourceNodeID: "server-001",
		TargetNodeID: "edge-b",
		DatabaseName: "nodebridge",
		TableName:    "sync_node_config",
		PrimaryKey:   map[string]any{"node_id": "edge-b"},
		After: map[string]any{
			"mysql_host":      "127.0.0.1",
			"mysql_port":      3308,
			"mysql_database":  "scada_edge",
			"mysql_username":  "sync_user",
			"cdc_type":        "canal",
			"cdc_filter":      "scada_edge\\..*",
			"cdc_batch_size":  1000,
			"cdc_destination": "edge-b",
			"rule_version":    7,
		},
		CreatedAt: time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
		EventTime: time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
		TraceID:   "trace-config",
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

func incomingRuntime(messages []*fakeMessage) []rabbitmq.IncomingMessage {
	result := make([]rabbitmq.IncomingMessage, 0, len(messages))
	for _, msg := range messages {
		result = append(result, msg)
	}
	return result
}
