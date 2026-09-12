package syncruntime

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
)

type downlinkTestWorker struct {
	batches [][]mapper.MappedEvent
	apply   func([]mapper.MappedEvent) (apply.BatchResult, error)
}

func (w *downlinkTestWorker) Apply(context.Context, mapper.MappedEvent) (apply.Result, error) {
	return apply.Result{}, errors.New("unexpected single apply")
}
func (w *downlinkTestWorker) ApplyBatch(_ context.Context, events []mapper.MappedEvent) (apply.BatchResult, error) {
	w.batches = append(w.batches, append([]mapper.MappedEvent(nil), events...))
	if w.apply != nil {
		return w.apply(events)
	}
	result := apply.BatchResult{}
	for _, evt := range events {
		result.Results = append(result.Results, apply.Result{EventID: evt.Event.EventID})
	}
	return result, nil
}

type configStoreFunc func(syncstore.NodeConfig) error

func (f configStoreFunc) UpsertNodeConfig(_ context.Context, c syncstore.NodeConfig) error {
	return f(c)
}

func downlinkMessages(t *testing.T, events ...event.SyncEvent) []*fakeMessage {
	t.Helper()
	var messages []*fakeMessage
	for _, evt := range events {
		messages = append(messages, &fakeMessage{body: mustJSON(t, evt)})
	}
	return messages
}
func requirePrefixAcks(t *testing.T, messages []*fakeMessage, prefix int) {
	t.Helper()
	for i, msg := range messages {
		if i < prefix {
			if !msg.acked || msg.nacked {
				t.Fatalf("message %d not ACK-only", i)
			}
		} else if msg.acked || !msg.nacked || !msg.requeue {
			t.Fatalf("message %d not requeued", i)
		}
	}
}

func TestDownlinkUsesBatchCommitAndLocalMappedDatabase(t *testing.T) {
	messages := downlinkMessages(t, sampleEventWithID("a", 1), sampleEventWithID("b", 1), sampleEventWithID("c", 2))
	w := &downlinkTestWorker{apply: func(events []mapper.MappedEvent) (apply.BatchResult, error) {
		for _, msg := range messages {
			if msg.acked {
				t.Fatal("ACK before commit")
			}
		}
		for i, evt := range events {
			if evt.TargetDatabase != "local_edge" || evt.Event.DatabaseName != "local_edge" || evt.SourceDatabase != "scada_edge" || evt.TargetTable != "device_settings" || evt.TargetPrimaryKey["setting_id"] == nil {
				t.Fatalf("mapping lost at %d: %+v", i, evt)
			}
		}
		return apply.BatchResult{Results: make([]apply.Result, len(events))}, nil
	}}
	result, err := (EdgeDownlinkBatchRuntime{Source: &fakeBatchSource{messages: incomingRuntime(messages)}, Rules: sampleRules(), Worker: w, TargetDatabaseOverride: "local_edge"}).RunOnce(context.Background())
	if err != nil || result.Count != 3 || result.EventID != "c" || len(w.batches) != 1 {
		t.Fatalf("result=%+v batches=%d err=%v", result, len(w.batches), err)
	}
	if w.batches[0][0].Event.EventID != "a" || w.batches[0][1].Event.EventID != "b" {
		t.Fatal("same-key order changed")
	}
	requirePrefixAcks(t, messages, 3)
}

func TestDownlinkConfigurationIsCommitBarrier(t *testing.T) {
	messages := downlinkMessages(t, sampleEventWithID("a", 1), sampleEventWithID("b", 2), sampleConfigEvent(), sampleEventWithID("c", 3))
	var order []string
	w := &downlinkTestWorker{apply: func(events []mapper.MappedEvent) (apply.BatchResult, error) {
		for _, evt := range events {
			order = append(order, evt.Event.EventID)
		}
		return apply.BatchResult{Results: make([]apply.Result, len(events))}, nil
	}}
	store := configStoreFunc(func(syncstore.NodeConfig) error { order = append(order, "config"); return nil })
	_, err := (EdgeDownlinkBatchRuntime{Source: &fakeBatchSource{messages: incomingRuntime(messages)}, Rules: sampleRules(), Worker: w, ConfigStore: store}).RunOnce(context.Background())
	if err != nil || len(w.batches) != 2 || !reflect.DeepEqual(order, []string{"a", "b", "config", "c"}) {
		t.Fatalf("order=%v err=%v", order, err)
	}
	requirePrefixAcks(t, messages, 4)
}

func TestDownlinkFailureAcksOnlyCommittedPrefix(t *testing.T) {
	for _, committed := range []int{0, 1, 2} {
		t.Run(string(rune('0'+committed)), func(t *testing.T) {
			messages := downlinkMessages(t, sampleEventWithID("a", 1), sampleEventWithID("b", 2), sampleEventWithID("c", 3))
			w := &downlinkTestWorker{apply: func([]mapper.MappedEvent) (apply.BatchResult, error) {
				return apply.BatchResult{Results: make([]apply.Result, committed)}, errors.New("transaction failure")
			}}
			result, err := (EdgeDownlinkBatchRuntime{Source: &fakeBatchSource{messages: incomingRuntime(messages)}, Consumer: rabbitmq.Consumer{RequeueOnError: true}, Rules: sampleRules(), Worker: w}).RunOnce(context.Background())
			if err == nil || result.EventID != []string{"a", "b", "c"}[committed] {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			requirePrefixAcks(t, messages, committed)
		})
	}
}

func TestDownlinkParseAndConfigFailuresPreservePriorCommit(t *testing.T) {
	for _, kind := range []string{"json", "rule", "config", "missing_config_store", "compact"} {
		t.Run(kind, func(t *testing.T) {
			messages := downlinkMessages(t, sampleEventWithID("a", 1), sampleEventWithID("bad", 2), sampleEventWithID("c", 3))
			set := sampleRules()
			var store NodeConfigStore
			switch kind {
			case "json":
				messages[1].body = []byte("{")
			case "rule":
				bad := sampleEventWithID("bad", 2)
				bad.TableName = "unknown_table"
				messages[1].body = mustJSON(t, bad)
			case "config", "missing_config_store":
				messages[1].body = mustJSON(t, sampleConfigEvent())
				if kind == "config" {
					store = &fakeConfigStore{err: errors.New("config write failure")}
				}
			case "compact":
				second := set.Rules[0]
				second.TableName = "compact_table"
				second.SyncMode = rules.SyncModeCRUDCompact
				set.Rules = append(set.Rules, second)
				bad := sampleEventWithID("bad", 2)
				bad.TableName = "compact_table"
				messages[1].body = mustJSON(t, bad)
			}
			w := &downlinkTestWorker{}
			_, err := (EdgeDownlinkBatchRuntime{Source: &fakeBatchSource{messages: incomingRuntime(messages)}, Consumer: rabbitmq.Consumer{RequeueOnError: true}, Rules: set, Worker: w, ConfigStore: store}).RunOnce(context.Background())
			if err == nil || len(w.batches) != 1 || len(w.batches[0]) != 1 {
				t.Fatalf("batches=%v err=%v", w.batches, err)
			}
			requirePrefixAcks(t, messages, 1)
		})
	}
}

func TestDownlinkDisabledRuleRetainsQueuedDDLAndRows(t *testing.T) {
	set := sampleRules()
	set.Rules[0].SchemaSync.AddColumns = true
	ignored := set.Rules[0]
	ignored.TableName = "ignored_table"
	ignored.Enable = false
	set.Rules = append(set.Rules, ignored)
	skip := sampleEventWithID("skip", 0)
	skip.TableName = "ignored_table"
	messages := downlinkMessages(t, skip, sampleEventWithID("a", 1), sampleSchemaEvent(event.TypeAddColumn), sampleEventWithID("b", 1), skip)
	w := &downlinkTestWorker{}
	_, err := (EdgeDownlinkBatchRuntime{Source: &fakeBatchSource{messages: incomingRuntime(messages)}, Consumer: rabbitmq.Consumer{RequeueOnError: true}, Rules: set, Worker: w}).RunOnce(context.Background())
	if err == nil || len(w.batches) != 0 {
		t.Fatalf("batches=%v err=%v", w.batches, err)
	}
	requirePrefixAcks(t, messages, 0)
}

func TestDownlinkDisabledRuleCommitsOnlyPrecedingPrefix(t *testing.T) {
	set := sampleRules()
	ignored := set.Rules[0]
	ignored.TableName = "ignored_table"
	ignored.Enable = false
	set.Rules = append(set.Rules, ignored)
	skip := sampleEventWithID("skip", 0)
	skip.TableName = "ignored_table"
	messages := downlinkMessages(t, sampleEventWithID("a", 1), skip, sampleEventWithID("b", 2), skip)
	w := &downlinkTestWorker{}
	result, err := (EdgeDownlinkBatchRuntime{Source: &fakeBatchSource{messages: incomingRuntime(messages)}, Consumer: rabbitmq.Consumer{RequeueOnError: true}, Rules: set, Worker: w}).RunOnce(context.Background())
	if err == nil || result.EventID != "skip" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	requirePrefixAcks(t, messages, 1)
}
