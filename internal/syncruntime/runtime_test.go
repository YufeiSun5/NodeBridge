package syncruntime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/dbgovernance"
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

func TestEdgeUploadBatchRuntimeUsesBatchPublisherInOrder(t *testing.T) {
	messages := []*fakeMessage{
		{body: mustJSON(t, sampleEventWithID("evt-001", 1))},
		{body: mustJSON(t, sampleEventWithID("evt-002", 2))},
		{body: mustJSON(t, sampleEventWithID("evt-003", 3))},
	}
	publisher := &fakeBatchPublisher{}
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
	if len(publisher.batches) != 1 || len(publisher.batches[0]) != 3 {
		t.Fatalf("expected one three-message batch, got %+v", publisher.batches)
	}
	for i, req := range publisher.batches[0] {
		if string(req.Body) != string(messages[i].body) {
			t.Fatalf("request %d out of order", i)
		}
	}
	for i, msg := range messages {
		if !msg.acked || msg.nacked {
			t.Fatalf("message %d expected ack only, got %+v", i, msg)
		}
	}
}

func TestEdgeUploadBatchRuntimeBatchPublisherFailureNacksAll(t *testing.T) {
	messages := []*fakeMessage{
		{body: mustJSON(t, sampleEventWithID("evt-001", 1))},
		{body: mustJSON(t, sampleEventWithID("evt-002", 2))},
		{body: mustJSON(t, sampleEventWithID("evt-003", 3))},
	}
	result, err := (EdgeUploadBatchRuntime{
		Source:        &fakeBatchSource{messages: incomingRuntime(messages)},
		Publisher:     &fakeBatchPublisher{err: errors.New("broker down")},
		Consumer:      rabbitmq.Consumer{RequeueOnError: true},
		MaxBatch:      3,
		FlushInterval: time.Hour,
	}).RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected batch publish failure")
	}
	if result.EventID != "evt-003" || result.Action != "failed" {
		t.Fatalf("unexpected result %+v", result)
	}
	for i, msg := range messages {
		if msg.acked || !msg.nacked || !msg.requeue {
			t.Fatalf("message %d should be requeue nacked, got %+v", i, msg)
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

func TestServerIngressRuntimeAppliesAndDispatchesMappedAddColumn(t *testing.T) {
	evt := sampleSchemaEvent(event.TypeAddColumn)
	msg := &fakeMessage{body: mustJSON(t, evt)}
	worker := &fakeWorker{}
	dispatcher := &fakeDispatcher{}
	set := sampleRules()
	set.Rules[0].SourceNodeIDs = []string{"edge-a"}
	set.Rules[0].DispatchTarget = rules.DispatchActiveEdges
	set.Rules[0].SchemaSync.AddColumns = true
	set.Rules[0].IncludeColumns = append(set.Rules[0].IncludeColumns, "source_note")
	set.Rules[0].ColumnMappings = append(set.Rules[0].ColumnMappings, rules.ColumnMapping{SourceColumn: "source_note", TargetColumn: "target_note"})

	result, err := (ServerIngressRuntime{
		Source: &fakeSource{msg: msg, ok: true}, Rules: set, Worker: worker,
		Dispatcher: dispatcher, EdgeNodes: []string{"edge-a", "edge-b"},
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "applied" || result.DispatchCount != 1 || len(worker.events) != 1 {
		t.Fatalf("unexpected result=%+v applied=%d", result, len(worker.events))
	}
	mapped := worker.events[0]
	if mapped.Event.SchemaChange == nil || mapped.Event.SchemaChange.Column.Name != "target_note" || mapped.TargetTable != "device_settings" {
		t.Fatalf("unexpected mapped schema event %+v", mapped)
	}
	if len(dispatcher.targets) != 1 || dispatcher.targets[0] != "edge-b" || !msg.acked || msg.nacked {
		t.Fatalf("unexpected dispatch=%+v ack=%t nack=%t", dispatcher.targets, msg.acked, msg.nacked)
	}
}

func TestServerIngressRuntimeAcksDisabledSchemaChangeWithoutApplying(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleSchemaEvent(event.TypeAddColumn))}
	worker := &fakeWorker{}
	dispatcher := &fakeDispatcher{}

	result, err := (ServerIngressRuntime{
		Source: &fakeSource{msg: msg, ok: true}, Rules: sampleRules(), Worker: worker,
		Dispatcher: dispatcher, EdgeNodes: []string{"edge-a", "edge-b"},
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "applied" || len(worker.events) != 0 || len(dispatcher.targets) != 0 || !msg.acked || msg.nacked {
		t.Fatalf("disabled schema change was not safely suppressed: result=%+v applied=%d dispatch=%+v ack=%t nack=%t", result, len(worker.events), dispatcher.targets, msg.acked, msg.nacked)
	}
}

func TestServerIngressRuntimeSuppressesUnscopedDropColumn(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleSchemaEvent(event.TypeDropColumn))}
	worker := &fakeWorker{}
	dispatcher := &fakeDispatcher{}
	set := sampleRules()
	set.Rules[0].SchemaSync.DropColumns = true
	set.Rules[0].DispatchTarget = rules.DispatchActiveEdges

	result, err := (ServerIngressRuntime{
		Source: &fakeSource{msg: msg, ok: true}, Rules: set, Worker: worker,
		Dispatcher: dispatcher, EdgeNodes: []string{"edge-a", "edge-b"},
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if len(worker.events) != 0 || len(dispatcher.targets) != 0 || !msg.acked || msg.nacked {
		t.Fatalf("unscoped DROP was not suppressed: result=%+v applied=%d dispatch=%+v ack=%t nack=%t", result, len(worker.events), dispatcher.targets, msg.acked, msg.nacked)
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
	set.Rules[0].DispatchTarget = rules.DispatchAuto
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
	set.Rules[0].DispatchTarget = rules.DispatchAuto
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

func TestServerIngressBatchRuntimeUsesOrderedBatchWorker(t *testing.T) {
	messages := []*fakeMessage{
		{body: mustJSON(t, sampleEventWithID("evt-001", 1))},
		{body: mustJSON(t, sampleEventWithID("evt-002", 2))},
	}
	worker := &fakeBatchWorker{}
	store := &fakeEventStore{}
	set := sampleRules()
	set.Rules[0].Direction = rules.DirectionEdgeToServer
	set.Rules[0].DispatchTarget = rules.DispatchAuto

	result, err := (ServerIngressBatchRuntime{
		Source:        &fakeBatchSource{messages: incomingRuntime(messages)},
		Rules:         set,
		Worker:        worker,
		EventStore:    store,
		MaxBatch:      2,
		FlushInterval: time.Hour,
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Count != 2 || result.EventID != "evt-002" || result.Action != "applied" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(worker.events) != 2 || worker.events[0].Event.EventID != "evt-001" || worker.events[1].Event.EventID != "evt-002" {
		t.Fatalf("unexpected batch apply order %+v", worker.events)
	}
	if len(store.records) != 2 || store.records[0].Status != syncstore.StatusSuccess || store.records[1].Status != syncstore.StatusSuccess {
		t.Fatalf("expected success-only batch event logs, got %+v", store.records)
	}
	if !store.records[0].SkipPayload || !store.records[1].SkipPayload {
		t.Fatalf("EDGE_TO_SERVER no-dispatch batch logs should skip payload, got %+v", store.records)
	}
	for i, msg := range messages {
		if !msg.acked || msg.nacked {
			t.Fatalf("message %d should be acked after batch commit, got %+v", i, msg)
		}
	}
}

func TestServerIngressBatchRuntimeNacksFromFailedApply(t *testing.T) {
	messages := []*fakeMessage{
		{body: mustJSON(t, sampleEventWithID("evt-001", 1))},
		{body: mustJSON(t, sampleEventWithID("evt-002", 2))},
		{body: mustJSON(t, sampleEventWithID("evt-003", 3))},
	}
	set := sampleRules()
	set.Rules[0].Direction = rules.DirectionEdgeToServer
	result, err := (ServerIngressBatchRuntime{
		Source:        &fakeBatchSource{messages: incomingRuntime(messages)},
		Consumer:      rabbitmq.Consumer{RequeueOnError: true},
		Rules:         set,
		Worker:        &fakeBatchWorker{err: errors.New("mysql down"), successCount: 1},
		MaxBatch:      3,
		FlushInterval: time.Hour,
	}).RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected batch apply failure")
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

func TestServerIngressBatchRuntimePersistsCommittedPrefixOnFailedApply(t *testing.T) {
	messages := []*fakeMessage{
		{body: mustJSON(t, sampleEventWithID("evt-001", 1))},
		{body: mustJSON(t, sampleEventWithID("evt-002", 2))},
	}
	set := sampleRules()
	set.Rules[0].Direction = rules.DirectionEdgeToServer
	store := &fakeEventStore{}
	result, err := (ServerIngressBatchRuntime{
		Source:        &fakeBatchSource{messages: incomingRuntime(messages)},
		Consumer:      rabbitmq.Consumer{RequeueOnError: true},
		Rules:         set,
		Worker:        &fakeBatchWorker{err: errors.New("mysql down"), successCount: 1},
		EventStore:    store,
		MaxBatch:      2,
		FlushInterval: time.Hour,
	}).RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected batch apply failure")
	}
	if result.Action != "failed" || result.EventID != "evt-002" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(store.records) != 1 || store.records[0].Event.EventID != "evt-001" || store.records[0].Status != syncstore.StatusSuccess {
		t.Fatalf("expected committed prefix event log, got %+v", store.records)
	}
	if !messages[0].acked || messages[1].acked || !messages[1].nacked {
		t.Fatalf("unexpected ack state first=%+v second=%+v", messages[0], messages[1])
	}
}

func TestApplyBatchWithLanesKeepsSamePrimaryKeyInOrder(t *testing.T) {
	events := []mapper.MappedEvent{
		mappedRuntimeEvent("evt-001", 1),
		mappedRuntimeEvent("evt-002", 2),
		mappedRuntimeEvent("evt-003", 1),
	}
	worker := &laneRecordingBatchWorker{}
	result, err := applyBatchWithLanes(context.Background(), worker, events, 4)
	if err != nil {
		t.Fatalf("applyBatchWithLanes returned error: %v", err)
	}
	if len(result.Results) != 3 {
		t.Fatalf("expected 3 results, got %+v", result)
	}
	if worker.orderForPK("setting_id=1") != "evt-001,evt-003" {
		t.Fatalf("same pk events were not ordered in one lane, batches=%+v", worker.batches)
	}
}

func TestApplyBatchWithLanesFallsBackForCompactMode(t *testing.T) {
	events := []mapper.MappedEvent{
		mappedRuntimeEvent("evt-001", 1),
		mappedRuntimeEvent("evt-002", 1),
	}
	events[0].SyncMode = rules.SyncModeCRUDCompact
	events[1].SyncMode = rules.SyncModeCRUDCompact
	worker := &laneRecordingBatchWorker{}
	if _, err := applyBatchWithLanes(context.Background(), worker, events, 4); err != nil {
		t.Fatalf("applyBatchWithLanes returned error: %v", err)
	}
	if len(worker.batches) != 1 || len(worker.batches[0]) != 2 {
		t.Fatalf("compact mode should use one ordered batch, got %+v", worker.batches)
	}
}

func TestServerIngressBatchRuntimeRejectsCompactWhenDisabled(t *testing.T) {
	messages := []*fakeMessage{{body: mustJSON(t, sampleEventWithID("evt-001", 1))}}
	set := sampleRules()
	set.Rules[0].SyncMode = rules.SyncModeCRUDCompact
	result, err := (ServerIngressBatchRuntime{
		Source:        &fakeBatchSource{messages: incomingRuntime(messages)},
		Consumer:      rabbitmq.Consumer{RequeueOnError: true},
		Rules:         set,
		Worker:        &fakeBatchWorker{},
		MaxBatch:      1,
		FlushInterval: time.Hour,
	}).RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected compact mode gate error")
	}
	if result.Action != "failed" || messages[0].acked || !messages[0].nacked || !messages[0].requeue {
		t.Fatalf("unexpected result=%+v message=%+v", result, messages[0])
	}
}

func TestServerIngressBatchRuntimeAllowsCompactWhenEnabled(t *testing.T) {
	messages := []*fakeMessage{{body: mustJSON(t, sampleEventWithID("evt-001", 1))}}
	set := sampleRules()
	set.Rules[0].SyncMode = rules.SyncModeCRUDCompact
	worker := &fakeBatchWorker{}
	result, err := (ServerIngressBatchRuntime{
		Source:           &fakeBatchSource{messages: incomingRuntime(messages)},
		Consumer:         rabbitmq.Consumer{RequeueOnError: true},
		Rules:            set,
		Worker:           worker,
		MaxBatch:         1,
		FlushInterval:    time.Hour,
		AllowCRUDCompact: true,
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Action != "applied" || !messages[0].acked || messages[0].nacked {
		t.Fatalf("unexpected result=%+v message=%+v", result, messages[0])
	}
	if len(worker.events) != 1 || worker.events[0].SyncMode != rules.SyncModeCRUDCompact {
		t.Fatalf("expected compact event to reach worker, got %+v", worker.events)
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

func TestServerIngressRuntimeDisabledRuleRequeuesWithoutApply(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	set := sampleRules()
	set.Rules[0].Enable = false
	worker := &fakeWorker{}
	runtime := ServerIngressRuntime{
		Source:   &fakeSource{msg: msg, ok: true},
		Rules:    set,
		Worker:   worker,
		Consumer: rabbitmq.Consumer{RequeueOnError: true},
	}

	result, err := runtime.RunOnce(context.Background())
	if err == nil {
		t.Fatal("disabled rule was silently accepted")
	}
	if result.Action != "failed" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(worker.events) != 0 {
		t.Fatalf("disabled rule should not apply, got %d", len(worker.events))
	}
	if msg.acked || !msg.nacked || !msg.requeue {
		t.Fatal("disabled rule must retain the message")
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

func TestEdgeDownlinkRuntimeDisabledRuleRequeuesWithoutApply(t *testing.T) {
	msg := &fakeMessage{body: mustJSON(t, sampleEvent())}
	set := sampleRules()
	set.Rules[0].Enable = false
	worker := &fakeWorker{}
	runtime := EdgeDownlinkRuntime{
		Source:   &fakeSource{msg: msg, ok: true},
		Rules:    set,
		Worker:   worker,
		Consumer: rabbitmq.Consumer{RequeueOnError: true},
	}

	result, err := runtime.RunOnce(context.Background())
	if err == nil {
		t.Fatal("disabled rule was silently accepted")
	}
	if result.Action != "failed" {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(worker.events) != 0 {
		t.Fatalf("disabled rule should not apply, got %d", len(worker.events))
	}
	if msg.acked || !msg.nacked || !msg.requeue {
		t.Fatal("disabled rule must retain the message")
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

type fakeBatchPublisher struct {
	fakePublisher
	batches [][]rabbitmq.PublishRequest
	err     error
}

func (p *fakeBatchPublisher) PublishBatch(ctx context.Context, reqs []rabbitmq.PublishRequest) error {
	copied := append([]rabbitmq.PublishRequest(nil), reqs...)
	p.batches = append(p.batches, copied)
	return p.err
}

type fakeWorker struct {
	events []mapper.MappedEvent
	err    error
}

type fakeBatchWorker struct {
	events       []mapper.MappedEvent
	err          error
	successCount int
}

type laneRecordingBatchWorker struct {
	mu      sync.Mutex
	batches [][]mapper.MappedEvent
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

func (w *fakeBatchWorker) Apply(ctx context.Context, evt mapper.MappedEvent) (apply.Result, error) {
	w.events = append(w.events, evt)
	return apply.Result{EventID: evt.Event.EventID, SourceTable: evt.SourceTable, TargetTable: evt.TargetTable}, w.err
}

func (w *fakeBatchWorker) ApplyBatch(ctx context.Context, events []mapper.MappedEvent) (apply.BatchResult, error) {
	w.events = append(w.events, events...)
	limit := len(events)
	if w.err != nil {
		limit = w.successCount
	}
	results := make([]apply.Result, 0, limit)
	for i := 0; i < limit && i < len(events); i++ {
		evt := events[i]
		results = append(results, apply.Result{EventID: evt.Event.EventID, SourceTable: evt.SourceTable, TargetTable: evt.TargetTable})
	}
	return apply.BatchResult{Results: results}, w.err
}

func (w *laneRecordingBatchWorker) Apply(ctx context.Context, evt mapper.MappedEvent) (apply.Result, error) {
	return apply.Result{EventID: evt.Event.EventID, SourceTable: evt.SourceTable, TargetTable: evt.TargetTable}, nil
}

func (w *laneRecordingBatchWorker) ApplyBatch(ctx context.Context, events []mapper.MappedEvent) (apply.BatchResult, error) {
	copied := append([]mapper.MappedEvent(nil), events...)
	w.mu.Lock()
	w.batches = append(w.batches, copied)
	w.mu.Unlock()

	results := make([]apply.Result, 0, len(events))
	for _, evt := range events {
		results = append(results, apply.Result{EventID: evt.Event.EventID, SourceTable: evt.SourceTable, TargetTable: evt.TargetTable})
	}
	return apply.BatchResult{Results: results}, nil
}

func (w *laneRecordingBatchWorker) orderForPK(pk string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var ids []string
	for _, batch := range w.batches {
		for _, evt := range batch {
			if pkValue(evt.TargetPrimaryKey) == pk {
				ids = append(ids, evt.Event.EventID)
			}
		}
	}
	return strings.Join(ids, ",")
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

func sampleSchemaEvent(operation string) event.SyncEvent {
	evt := sampleEvent()
	evt.EventID = "evt-schema-001"
	evt.EventType = operation
	evt.PrimaryKey, evt.Before, evt.After = nil, nil, nil
	evt.SchemaChange = &dbgovernance.SchemaChange{
		Operation: operation,
		Column: dbgovernance.ColumnDefinition{
			Name:     "source_note",
			Type:     "varchar(64)",
			Nullable: true,
		},
	}
	return evt
}

func mappedRuntimeEvent(eventID string, id int) mapper.MappedEvent {
	evt := sampleEventWithID(eventID, id)
	return mapper.MappedEvent{
		Event:          evt,
		SourceDatabase: evt.DatabaseName,
		SourceTable:    evt.TableName,
		TargetDatabase: "scada_center",
		TargetTable:    "device_settings",
		SyncMode:       rules.SyncModeOrderedCRUD,
		TargetPrimaryKey: map[string]any{
			"setting_id": id,
		},
		TargetAfter: map[string]any{
			"setting_id":    id,
			"display_name":  "Pump A",
			"setting_value": "ON",
		},
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
			Direction:          rules.DirectionEdgeToServer,
			DispatchTarget:     rules.DispatchActiveEdges,
			ConflictPolicy:     rules.ConflictNone,
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
