package syncruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/dbgovernance"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/normalizer"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
)

func TestServerCanalBatchPersistsConfirmsAndCommitsInOrder(t *testing.T) {
	trace := []string{}
	source := newRewindingCanalSource()
	source.trace = &trace
	store := &recordingDispatchStore{trace: &trace}
	dispatcher := &recordingBatchDispatcher{trace: &trace}
	nodes := &countedNodeStore{nodes: []string{"edge-a", "server-001", "", "edge-b"}}
	runtime := newBatchDispatchRuntime(source, store, dispatcher)
	runtime.NodeStore, runtime.EdgeNodes = nodes, nil
	result, err := runtime.RunOnce(context.Background())
	if err != nil || result.Action != "dispatched" || result.DispatchCount != 4 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !reflect.DeepEqual(trace, []string{"PENDING", "publish", "SUCCESS", "commit-1"}) {
		t.Fatalf("unsafe processing order: %v", trace)
	}
	if nodes.calls != 1 || len(store.batches) != 2 || len(store.batches[0]) != 2 {
		t.Fatalf("nodes=%d log batches=%v", nodes.calls, store.batches)
	}
	for i, request := range dispatcher.requests {
		wantNode := []string{"edge-a", "edge-b"}[i%2]
		if request.TargetNodeID != wantNode || request.Event.PrimaryKey["id"] != i/2+1 {
			t.Fatalf("fanout order changed at %d: %+v", i, request)
		}
	}
	for batchIndex, batch := range store.batches {
		for _, record := range batch {
			if record.Event.DatabaseName != "scada_center" || record.TargetDatabaseName != "scada_edge" || record.TargetTableName != "device_config" {
				t.Fatalf("lost source/target mapping: %+v", record)
			}
			if (batchIndex == 0) != record.AppliedAt.IsZero() {
				t.Fatalf("invalid applied time: %+v", record)
			}
		}
	}
}

func TestServerCanalIndependentUpdatesKeepRepeatedKeyAndDeleteBarriers(t *testing.T) {
	source := newRewindingCanalSource()
	base := source.batches[0][0]
	var changes []cdc.ChangeEvent
	for i, id := range []int{1, 2, 3, 1, 4, 4, 5, 6} {
		change := base
		change.Operation = cdc.OperationUpdate
		change.PrimaryKey = map[string]any{"id": id}
		change.After = map[string]any{"id": id, "value": i}
		change.BinlogPos = uint32(i + 20)
		if i == 5 {
			change.Operation = cdc.OperationDelete
		}
		changes = append(changes, change)
	}
	source.batches = [][]cdc.ChangeEvent{changes}
	dispatcher := &recordingBatchDispatcher{}
	runtime := newBatchDispatchRuntime(source, nil, dispatcher)
	if _, err := runtime.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dispatcher.batchSizes, []int{3, 2, 1, 2}) {
		t.Fatalf("unsafe key barriers: %v", dispatcher.batchSizes)
	}
}

func TestServerCanalUpdateConfirmationGroupIsBounded(t *testing.T) {
	source := newRewindingCanalSource()
	base := source.batches[0][0]
	changes := make([]cdc.ChangeEvent, 130)
	for i := range changes {
		change := base
		change.Operation = cdc.OperationUpdate
		change.PrimaryKey = map[string]any{"id": i}
		change.After = map[string]any{"id": i}
		change.BinlogPos = uint32(i + 20)
		changes[i] = change
	}
	source.batches = [][]cdc.ChangeEvent{changes}
	dispatcher := &recordingBatchDispatcher{}
	runtime := newBatchDispatchRuntime(source, nil, dispatcher)
	if _, err := runtime.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dispatcher.batchSizes, []int{64, 64, 2}) {
		t.Fatalf("unbounded updates: %v", dispatcher.batchSizes)
	}
}

func TestServerCanalSelectiveNackInIndependentGroupCannotPassSameKey(t *testing.T) {
	source := newRewindingCanalSource()
	base := source.batches[0][0]
	var changes []cdc.ChangeEvent
	for i, id := range []int{1, 2, 1} {
		change := base
		change.Operation = cdc.OperationUpdate
		change.PrimaryKey = map[string]any{"id": id}
		change.After = map[string]any{"id": id, "value": i + 1}
		change.BinlogPos = uint32(i + 10)
		changes = append(changes, change)
	}
	source.batches = [][]cdc.ChangeEvent{changes}
	dispatcher := &independentNackDispatcher{applied: map[string]bool{}, values: map[string]int{}}
	runtime := newBatchDispatchRuntime(source, nil, dispatcher)
	if _, err := runtime.RunOnce(context.Background()); err == nil || source.acked != 0 {
		t.Fatal("NACK committed source")
	}
	if dispatcher.values["id=1"] != 0 || dispatcher.values["id=2"] != 2 {
		t.Fatalf("dependent update crossed rejected predecessor: %v", dispatcher.values)
	}
	if _, err := runtime.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if source.acked != 1 || dispatcher.values["id=1"] != 3 || dispatcher.values["id=2"] != 2 || len(dispatcher.applied) != 3 {
		t.Fatalf("retry reordered per-key values: %v", dispatcher.values)
	}
}

type independentNackDispatcher struct {
	fakeDispatcher
	rejected bool
	applied  map[string]bool
	values   map[string]int
}

func (d *independentNackDispatcher) DispatchBatch(_ context.Context, requests []DownlinkRequest) error {
	var err error
	for _, r := range requests {
		if !d.rejected {
			d.rejected = true
			err = errors.New("selective NACK")
			continue
		}
		if !d.applied[r.Event.EventID] {
			d.applied[r.Event.EventID] = true
			d.values[pkValue(r.Event.PrimaryKey)] = r.Event.After["value"].(int)
		}
	}
	return err
}

func TestServerCanalBatchRetriesBeforeFetchingLaterBatch(t *testing.T) {
	for _, stage := range []string{"normalize", "encode", "mapping", "nodes", "pending", "publish", "partial-publish", "success", "cancel", "commit"} {
		t.Run(stage, func(t *testing.T) {
			source := newRewindingCanalSource()
			store := &recordingDispatchStore{}
			dispatcher := &recordingBatchDispatcher{}
			runtime := newBatchDispatchRuntime(source, store, dispatcher)
			normal := runtime.Normalizer
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failure := errors.New("injected " + stage)
			switch stage {
			case "normalize":
				runtime.Normalizer = fakeNormalizer{err: failure}
			case "encode":
				evt := sampleServerEvent()
				evt.After["bad"] = func() {}
				runtime.Normalizer = fakeNormalizer{event: evt}
			case "mapping":
				runtime.Rules.Rules[0].TargetTableName = "invalid-table!"
			case "nodes":
				runtime.NodeStore = &fakeActiveNodeStore{err: failure}
				runtime.EdgeNodes = nil
			case "pending":
				store.failStatus = syncstore.StatusPending
			case "publish":
				dispatcher.err = failure
			case "partial-publish":
				runtime.Dispatcher = RoutingDownlinkDispatcher{Publisher: &fakePublisher{err: failure, failOnCall: 2}}
			case "success":
				store.failStatus = syncstore.StatusSuccess
			case "cancel":
				dispatcher.afterPublish = cancel
			case "commit":
				source.commitErr = failure
			}
			result, err := runtime.RunOnce(ctx)
			if err == nil || result.Action != "failed" || source.acked != 0 || runtime.started || source.stopCount != 1 {
				t.Fatalf("failure advanced source: result=%+v err=%v source=%+v", result, err, source)
			}
			runtime.Normalizer, runtime.Rules = normal, sampleServerRules()
			runtime.NodeStore, runtime.EdgeNodes = nil, []string{"edge-a"}
			runtime.Dispatcher = dispatcher
			dispatcher.err, dispatcher.afterPublish, dispatcher.requests = nil, nil, nil
			store.failStatus, source.commitErr = "", nil
			result, err = runtime.RunOnce(context.Background())
			if err != nil || result.Action != "dispatched" || source.acked != 1 || source.startCount != 2 {
				t.Fatalf("retry failed: result=%+v err=%v source=%+v", result, err, source)
			}
			if len(dispatcher.requests) != 2 || dispatcher.requests[0].Event.PrimaryKey["id"] != 1 {
				t.Fatalf("retry skipped first batch: %+v", dispatcher.requests)
			}
			first, _ := normal.Normalize(source.batches[0][0])
			if dispatcher.requests[0].Event.EventID != first.EventID {
				t.Fatal("retry changed stable event ID")
			}
			if _, err := runtime.RunOnce(context.Background()); err != nil || source.acked != 2 {
				t.Fatalf("later batch failed: %v", err)
			}
		})
	}
}

func TestCanalUploadRetriesFailedBatchBeforeLaterBatch(t *testing.T) {
	for _, stage := range []string{"normalize", "encode", "publish"} {
		t.Run(stage, func(t *testing.T) {
			source := newRewindingCanalSource()
			publisher := &fakeBatchPublisher{}
			normal := normalizer.New(normalizer.Options{NodeID: "server-001"})
			runtime := &CanalUploadRuntime{Source: source, Normalizer: normal, Publisher: publisher}
			switch stage {
			case "normalize":
				runtime.Normalizer = fakeNormalizer{err: errors.New("normalize failed")}
			case "encode":
				evt := sampleServerEvent()
				evt.After["bad"] = func() {}
				runtime.Normalizer = fakeNormalizer{event: evt}
			case "publish":
				publisher.err = errors.New("publish failed")
			}
			if _, err := runtime.RunOnce(context.Background()); err == nil || source.acked != 0 || source.stopCount != 1 {
				t.Fatalf("expected rollback after error: %v %+v", err, source)
			}
			runtime.Normalizer, publisher.err, publisher.batches = normal, nil, nil
			if _, err := runtime.RunOnce(context.Background()); err != nil || source.acked != 1 {
				t.Fatalf("expected first batch retry: %v %+v", err, source)
			}
			var evt event.SyncEvent
			if err := json.Unmarshal(publisher.batches[0][0].Body, &evt); err != nil || evt.PrimaryKey["id"] != json.Number("1") {
				t.Fatalf("wrong retried event: %+v %v", evt, err)
			}
		})
	}
}

func TestServerCanalBatchRefreshesEmptyActiveNodesOnNextBatch(t *testing.T) {
	source := newRewindingCanalSource()
	dispatcher := &recordingBatchDispatcher{}
	nodes := &countedNodeStore{}
	runtime := newBatchDispatchRuntime(source, nil, dispatcher)
	runtime.EdgeNodes, runtime.NodeStore = nil, nodes
	result, err := runtime.RunOnce(context.Background())
	if err != nil || result.Action != "suppressed" || nodes.calls != 1 || len(dispatcher.requests) != 0 || source.acked != 1 {
		t.Fatalf("unexpected no-target batch: %+v %v calls=%d", result, err, nodes.calls)
	}
	nodes.nodes = []string{"edge-new"}
	result, err = runtime.RunOnce(context.Background())
	if err != nil || result.DispatchCount != 1 || nodes.calls != 2 || dispatcher.requests[0].TargetNodeID != "edge-new" {
		t.Fatalf("did not refresh nodes: %+v %v calls=%d", result, err, nodes.calls)
	}
}

func TestServerCanalBatchSelectiveNackCannotReorderDependentUpdates(t *testing.T) {
	source := newRewindingCanalSource()
	for i := range source.batches[0] {
		change := &source.batches[0][i]
		change.PrimaryKey = map[string]any{"id": 1}
		change.After["id"], change.After["value"] = 1, i+1
	}
	dispatcher := &selectiveNackDispatcher{applied: make(map[string]bool)}
	runtime := newBatchDispatchRuntime(source, &recordingDispatchStore{}, dispatcher)
	result, err := runtime.RunOnce(context.Background())
	if err == nil || result.Action != "failed" || source.acked != 0 {
		t.Fatalf("expected uncommitted nack: %+v %v", result, err)
	}
	if len(dispatcher.applied) != 0 {
		t.Fatal("later dependent update was sent past a rejected predecessor")
	}
	if _, err := runtime.RunOnce(context.Background()); err != nil || source.acked != 1 {
		t.Fatalf("retry failed: %v", err)
	}
	if dispatcher.value != 2 || len(dispatcher.applied) != 2 {
		t.Fatalf("replay plus idempotency left stale state: value=%d applied=%v", dispatcher.value, dispatcher.applied)
	}
}

func TestServerCanalBatchesOnlyConsecutiveAppendInserts(t *testing.T) {
	source := newRewindingCanalSource()
	base := source.batches[0][0]
	var changes []cdc.ChangeEvent
	for i, op := range []cdc.Operation{cdc.OperationInsert, cdc.OperationInsert, cdc.OperationAddColumn, cdc.OperationInsert, cdc.OperationInsert, cdc.OperationDropColumn, cdc.OperationUpdate, cdc.OperationDelete, cdc.OperationInsert} {
		change := base
		change.Operation, change.BinlogPos = op, uint32(i+10)
		if op == cdc.OperationAddColumn || op == cdc.OperationDropColumn {
			change.SchemaChange = &dbgovernance.SchemaChange{Operation: string(op), Column: dbgovernance.ColumnDefinition{Name: "extra", Type: "INT", Nullable: true}}
		}
		changes = append(changes, change)
	}
	source.batches = [][]cdc.ChangeEvent{changes}
	dispatcher := &recordingBatchDispatcher{}
	runtime := newBatchDispatchRuntime(source, &recordingDispatchStore{}, dispatcher)
	rule := &runtime.Rules.Rules[0]
	rule.SyncMode = rules.SyncModeAppendOnly
	rule.SchemaSync = rules.SchemaSync{AddColumns: true, DropColumns: true}
	if _, err := runtime.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dispatcher.batchSizes, []int{2, 1, 2, 1, 1, 1, 1}) {
		t.Fatalf("crossed CRUD/DDL confirmation barrier: %v", dispatcher.batchSizes)
	}
}

func TestServerCanalBatchSuppressesDisabledUnmatchedAndNoDispatchRules(t *testing.T) {
	for _, mode := range []string{"disabled", "unmatched", "ignore", "no-dispatch"} {
		t.Run(mode, func(t *testing.T) {
			source := newRewindingCanalSource()
			store, dispatcher := &recordingDispatchStore{}, &recordingBatchDispatcher{}
			runtime := newBatchDispatchRuntime(source, store, dispatcher)
			rule := &runtime.Rules.Rules[0]
			switch mode {
			case "disabled":
				rule.Enable = false
			case "unmatched":
				rule.TableName = "another_table"
			case "ignore":
				rule.Direction = rules.DirectionIgnore
			case "no-dispatch":
				rule.DispatchTarget = rules.DispatchNone
			}
			result, err := runtime.RunOnce(context.Background())
			if err != nil || result.Action != "suppressed" || source.acked != 1 || len(store.batches) != 0 || len(dispatcher.requests) != 0 {
				t.Fatalf("suppression failed: %+v %v", result, err)
			}
		})
	}
}

func TestServerCanalBatchPreservesCRUDAndDDLGates(t *testing.T) {
	for _, allowDDL := range []bool{false, true} {
		t.Run(fmt.Sprint(allowDDL), func(t *testing.T) {
			source := newRewindingCanalSource()
			base := source.batches[0][0]
			changes := []cdc.ChangeEvent{}
			for i, op := range []cdc.Operation{cdc.OperationInsert, cdc.OperationAddColumn, cdc.OperationUpdate, cdc.OperationDropColumn, cdc.OperationDelete} {
				change := base
				change.Operation, change.BinlogPos = op, uint32(i+10)
				if op == cdc.OperationAddColumn || op == cdc.OperationDropColumn {
					change.SchemaChange = &dbgovernance.SchemaChange{Operation: string(op), Column: dbgovernance.ColumnDefinition{Name: "extra", Type: "INT", Nullable: true}}
				}
				changes = append(changes, change)
			}
			source.batches = [][]cdc.ChangeEvent{changes}
			dispatcher := &recordingBatchDispatcher{}
			runtime := newBatchDispatchRuntime(source, &recordingDispatchStore{}, dispatcher)
			rule := &runtime.Rules.Rules[0]
			rule.DispatchTarget, rule.DispatchNodeIDs = rules.DispatchSelectedEdges, []string{"edge-b"}
			rule.SchemaSync = rules.SchemaSync{AddColumns: allowDDL, DropColumns: allowDDL}
			if _, err := runtime.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			want := []string{"INSERT", "UPDATE", "DELETE"}
			if allowDDL {
				want = []string{"INSERT", "ADD_COLUMN", "UPDATE", "DROP_COLUMN", "DELETE"}
			}
			got := []string{}
			for _, request := range dispatcher.requests {
				got = append(got, request.Event.EventType)
				if request.TargetNodeID != "edge-b" {
					t.Fatalf("wrong selected edge: %+v", request)
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v want %v", got, want)
			}
		})
	}
}

type recordingDispatchStore struct {
	fakeEventStore
	batches    [][]syncstore.EventLogRecord
	trace      *[]string
	failStatus string
}

func (s *recordingDispatchStore) UpsertEventLogs(ctx context.Context, records []syncstore.EventLogRecord) error {
	s.batches = append(s.batches, append([]syncstore.EventLogRecord(nil), records...))
	if s.trace != nil {
		*s.trace = append(*s.trace, records[0].Status)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if records[0].Status == s.failStatus {
		return errors.New("injected log failure")
	}
	return nil
}

type recordingBatchDispatcher struct {
	fakeDispatcher
	requests     []DownlinkRequest
	batchSizes   []int
	trace        *[]string
	err          error
	afterPublish func()
}

func (d *recordingBatchDispatcher) DispatchBatch(ctx context.Context, requests []DownlinkRequest) error {
	d.requests = append(d.requests, requests...)
	d.batchSizes = append(d.batchSizes, len(requests))
	if d.trace != nil {
		*d.trace = append(*d.trace, "publish")
	}
	if d.afterPublish != nil {
		d.afterPublish()
	}
	return d.err
}

type selectiveNackDispatcher struct {
	fakeDispatcher
	rejected bool
	applied  map[string]bool
	value    int
}

func (d *selectiveNackDispatcher) DispatchBatch(ctx context.Context, requests []DownlinkRequest) error {
	var err error
	for _, request := range requests {
		if !d.rejected {
			d.rejected = true
			err = errors.New("broker selectively rejected first update")
			continue
		}
		if !d.applied[request.Event.EventID] {
			d.value = request.Event.After["value"].(int)
			d.applied[request.Event.EventID] = true
		}
	}
	return err
}

type countedNodeStore struct {
	nodes []string
	calls int
}

func (s *countedNodeStore) ListActiveEdgeNodeIDs(context.Context) ([]string, error) {
	s.calls++
	return s.nodes, nil
}

type rewindingCanalSource struct {
	fakeCanalBatchSource
	batches [][]cdc.ChangeEvent
	next    int
	acked   int
	trace   *[]string
}

func newRewindingCanalSource() *rewindingCanalSource {
	changes := make([]cdc.ChangeEvent, 3)
	for i := range changes {
		changes[i] = sampleServerChange()
		changes[i].PrimaryKey = map[string]any{"id": i + 1}
		changes[i].After["id"] = i + 1
		changes[i].BinlogFile, changes[i].BinlogPos = "mysql-bin.000001", uint32(i+4)
	}
	return &rewindingCanalSource{batches: [][]cdc.ChangeEvent{changes[:2], changes[2:]}}
}

func (s *rewindingCanalSource) Start(ctx context.Context) error {
	s.next = s.acked
	return s.fakeCanalBatchSource.Start(ctx)
}

func (s *rewindingCanalSource) FetchChangesOnce(context.Context) ([]cdc.ChangeEvent, cdc.Offset, error) {
	if s.next == len(s.batches) {
		return nil, cdc.Offset{}, nil
	}
	batch := s.batches[s.next]
	s.next++
	return batch, cdc.Offset{BatchID: int64(s.next)}, nil
}

func (s *rewindingCanalSource) Commit(ctx context.Context, offset cdc.Offset) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.commitErr != nil {
		return s.commitErr
	}
	s.acked = int(offset.BatchID)
	if s.trace != nil {
		*s.trace = append(*s.trace, fmt.Sprintf("commit-%d", offset.BatchID))
	}
	return nil
}

func newBatchDispatchRuntime(source CanalBatchSource, store EventLogStore, dispatcher DownlinkDispatcher) *ServerCanalDispatchRuntime {
	return &ServerCanalDispatchRuntime{
		Source: source, Normalizer: normalizer.New(normalizer.Options{NodeID: "server-001"}),
		Rules: sampleServerRules(), EventStore: store, Dispatcher: dispatcher, EdgeNodes: []string{"edge-a"},
	}
}
