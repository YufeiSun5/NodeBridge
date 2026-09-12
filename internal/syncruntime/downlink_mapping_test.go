package syncruntime

import (
	"context"
	"testing"

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
