package syncruntime

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestDirectDownlinkPreservesExplicitTargetAndSourceIdentity(t *testing.T) {
	for _, batch := range []bool{false, true} {
		set := sampleRules()
		set.Rules[0].Direction = rules.DirectionServerToEdge
		evt := sampleEvent()
		worker := &fakeWorker{}
		msg := &fakeMessage{body: mustJSON(t, evt)}
		var err error
		if batch {
			_, err = (EdgeDownlinkBatchRuntime{Source: &fakeBatchSource{messages: incomingRuntime([]*fakeMessage{msg})}, Rules: set, Worker: worker, TargetDatabaseOverride: "local_default"}).RunOnce(context.Background())
		} else {
			_, err = (EdgeDownlinkRuntime{Source: &fakeSource{msg: msg, ok: true}, Rules: set, Worker: worker, TargetDatabaseOverride: "local_default"}).RunOnce(context.Background())
		}
		if err != nil || len(worker.events) != 1 {
			t.Fatalf("batch=%v err=%v", batch, err)
		}
		mapped := worker.events[0]
		if mapped.TargetDatabase != "scada_center" || mapped.Event.DatabaseName != mapped.TargetDatabase || mapped.SourceDatabase != evt.DatabaseName || evt.DatabaseName != "scada_edge" {
			t.Fatalf("batch=%v wrong mapping: %+v", batch, mapped)
		}
	}
}

func TestBidirectionalDownlinkBusinessDatabaseAndRetry(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE"} {
			t.Run(fmt.Sprintf("batch=%v/%s", batch, operation), func(t *testing.T) {
				set := sampleRules()
				rule := &set.Rules[0]
				rule.DatabaseName, rule.TableName = "spindle_main", "central_standards"
				rule.TargetDatabaseName, rule.TargetTableName = "spindle_edge_ac01", "sys_detection_standards"
				rule.Direction, rule.ConflictPolicy, rule.DeleteMode = rules.DirectionBidirectional, rules.ConflictLastWriteWin, rules.DeleteHard
				*rule = rule.BindPairedRuntime()
				evt := sampleEvent()
				evt.DatabaseName, evt.TableName, evt.EventType = rule.DatabaseName, rule.TableName, operation
				evt.OriginNodeID, evt.SourceNodeID = "server-001", "server-001"
				evt.Before = evt.After
				if operation == "DELETE" {
					evt.After = nil
				}
				msg := &fakeMessage{body: mustJSON(t, evt)}
				worker := &fakeWorker{err: errors.New("temporary apply failure")}
				consumer := rabbitmq.Consumer{RequeueOnError: true}
				run := func() error {
					if batch {
						_, err := (EdgeDownlinkBatchRuntime{Source: &fakeBatchSource{messages: incomingRuntime([]*fakeMessage{msg})}, Rules: set, Worker: worker, Consumer: consumer, TargetDatabaseOverride: "scada_edge"}).RunOnce(context.Background())
						return err
					}
					_, err := (EdgeDownlinkRuntime{Source: &fakeSource{msg: msg, ok: true}, Rules: set, Worker: worker, Consumer: consumer, TargetDatabaseOverride: "scada_edge"}).RunOnce(context.Background())
					return err
				}
				var failure *EventFailure
				if err := run(); !errors.As(err, &failure) || failure.TargetDatabase != rule.TargetDatabaseName || failure.SourceDatabase != evt.DatabaseName || failure.TargetTable != rule.TargetTableName || msg.acked || !msg.nacked || !msg.requeue {
					t.Fatalf("failure=%+v err=%v message=%+v", failure, err, msg)
				}
				worker.err = nil
				if err := run(); err != nil || !msg.acked || len(worker.events) != 2 {
					t.Fatalf("retry err=%v message=%+v applied=%d", err, msg, len(worker.events))
				}
				for _, mapped := range worker.events {
					if mapped.TargetDatabase != rule.TargetDatabaseName || mapped.TargetTable != rule.TargetTableName || mapped.SourceDatabase != evt.DatabaseName || mapped.Event.DatabaseName != rule.TargetDatabaseName || mapped.Event.EventID != evt.EventID || mapped.TargetPrimaryKey["setting_id"] == nil {
						t.Fatalf("wrong mapping: %+v", mapped)
					}
				}
			})
		}
	}
}
