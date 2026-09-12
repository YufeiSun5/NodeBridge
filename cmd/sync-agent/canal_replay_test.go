package main

import (
	"context"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type replayLog map[string]bool

func (l replayLog) Exists(_ context.Context, id string) (bool, error) { return l[id], nil }

func TestCanalUploadWiresApplyLog(t *testing.T) {
	cfg := &appconfig.Config{Node: appconfig.NodeConfig{ID: "edge-001"}, CDC: appconfig.CDCConfig{CanalAddr: "127.0.0.1:11111", Destination: "edge-001", BatchSize: 1000}}
	set := &rules.RuleSet{Rules: []rules.SyncRule{{ID: "state", DatabaseName: "db", TableName: "state", Enable: true, Direction: rules.DirectionBidirectional}}}
	runtime, err := newCanalUploadRuntime(cfg, set, nil, nil, replayLog{"server-event": true})
	if err != nil {
		t.Fatal(err)
	}
	change := cdc.ChangeEvent{DatabaseName: "db", TableName: "state", Operation: cdc.OperationInsert, After: map[string]any{"last_event_id": "server-event", "updated_by_node": "server-001"}}
	if decision, err := runtime.Decider.ShouldUpload(context.Background(), change); err != nil || decision.Upload {
		t.Fatalf("replay was published: %+v err=%v", decision, err)
	}
	change.After["last_event_id"] = ""
	change.After["updated_by_node"] = "edge-001"
	if decision, err := runtime.Decider.ShouldUpload(context.Background(), change); err != nil || !decision.Upload {
		t.Fatalf("business change was suppressed: %+v err=%v", decision, err)
	}
	if _, err := newCanalUploadRuntime(cfg, set, nil, nil, nil); err == nil {
		t.Fatal("missing apply log accepted")
	}
}
