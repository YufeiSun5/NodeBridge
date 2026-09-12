package loop_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/dbgovernance"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestShouldUploadWhenRuleMissing(t *testing.T) {
	s := loop.NewSuppressor("edge-001", rules.RuleSet{}, fakeApplyLog{})

	decision := mustDecision(t, s, cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config"})

	if decision.Upload {
		t.Fatalf("expected no upload, got %+v", decision)
	}
	if decision.Reason != "table not in sync rules" {
		t.Fatalf("unexpected reason %q", decision.Reason)
	}
}

func TestShouldUploadWhenRuleDisabled(t *testing.T) {
	s := loop.NewSuppressor("edge-001", ruleSet(rules.SyncRule{Enable: false}), fakeApplyLog{})

	decision := mustDecision(t, s, change())

	if decision.Upload {
		t.Fatalf("expected disabled rule to suppress upload, got %+v", decision)
	}
}

func TestShouldUploadWhenIgnored(t *testing.T) {
	s := loop.NewSuppressor("edge-001", ruleSet(rules.SyncRule{
		Enable:    true,
		Direction: rules.DirectionIgnore,
	}), fakeApplyLog{})

	decision := mustDecision(t, s, change())

	if decision.Upload {
		t.Fatalf("expected ignored table to suppress upload, got %+v", decision)
	}
	if decision.Reason != "ignored table" {
		t.Fatalf("unexpected reason %q", decision.Reason)
	}
}

func TestShouldUploadSuppressesReplayedEvent(t *testing.T) {
	s := loop.NewSuppressor("edge-002", ruleSet(rules.SyncRule{
		Enable:    true,
		Direction: rules.DirectionBidirectional,
	}), fakeApplyLog{"evt-001": true})

	decision := mustDecision(t, s, cdc.ChangeEvent{
		DatabaseName: "scada_edge",
		TableName:    "device_config",
		After: map[string]any{
			"last_event_id":   "evt-001",
			"updated_by_node": "edge-001",
		},
	})

	if decision.Upload {
		t.Fatalf("expected replayed event to suppress upload, got %+v", decision)
	}
	if decision.Reason != "replayed sync event" {
		t.Fatalf("unexpected reason %q", decision.Reason)
	}
}

func TestMappedReplayMarkerAndSubsequentBusinessUpdate(t *testing.T) {
	s := loop.NewSuppressor("edge-002", ruleSet(rules.SyncRule{Enable: true, Direction: rules.DirectionBidirectional, ColumnMappings: []rules.ColumnMapping{{SourceColumn: "event_ref", TargetColumn: "last_event_id"}, {SourceColumn: "writer", TargetColumn: "updated_by_node"}}}), fakeApplyLog{"evt-current": true})
	change := cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config", After: map[string]any{"event_ref": "evt-current", "writer": "edge-001"}}
	if mustDecision(t, s, change).Upload {
		t.Fatal("replayed mapped metadata was uploaded")
	}
	change.After["event_ref"] = ""
	change.After["writer"] = "edge-002"
	if !mustDecision(t, s, change).Upload {
		t.Fatal("local business update was suppressed")
	}
}

func TestShouldUploadLocalBusinessChange(t *testing.T) {
	s := loop.NewSuppressor("edge-001", ruleSet(rules.SyncRule{
		Enable:    true,
		Direction: rules.DirectionBidirectional,
	}), fakeApplyLog{"evt-001": true})

	decision := mustDecision(t, s, cdc.ChangeEvent{
		DatabaseName: "scada_edge",
		TableName:    "device_config",
		After: map[string]any{
			"last_event_id":   "evt-001",
			"updated_by_node": "edge-001",
		},
	})

	if !decision.Upload {
		t.Fatalf("expected local change to upload, got %+v", decision)
	}
	if decision.Reason != "local business change" {
		t.Fatalf("unexpected reason %q", decision.Reason)
	}
}

func TestRetainedReplayMarkersDoNotDiscardLaterLocalUpdates(t *testing.T) {
	for _, mapped := range []bool{false, true} {
		rule := rules.SyncRule{Enable: true, Direction: rules.DirectionBidirectional}
		eventColumn, nodeColumn := "last_event_id", "updated_by_node"
		if mapped {
			eventColumn, nodeColumn = "event_ref", "writer"
			rule.ColumnMappings = []rules.ColumnMapping{{SourceColumn: eventColumn, TargetColumn: "last_event_id"}, {SourceColumn: nodeColumn, TargetColumn: "updated_by_node"}}
		}
		s := loop.NewSuppressor("edge-002", ruleSet(rule), failingApplyLog{errors.New("must not look up an unchanged marker")})
		change := cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config", Operation: cdc.OperationUpdate,
			Before: map[string]any{eventColumn: "remote-1", nodeColumn: "edge-001", "value": "old"},
			After:  map[string]any{eventColumn: "remote-1", nodeColumn: "edge-001", "value": "new"}}
		if !mustDecision(t, s, change).Upload {
			t.Fatal("local update retaining remote markers was discarded")
		}
		change.Before["value"] = "new"
		if !mustDecision(t, s, change).Upload {
			t.Fatal("same-value local update retaining markers was discarded")
		}
	}
}

func TestUpdateReplayRequiresMarkerTransitionAndFullBeforeImage(t *testing.T) {
	s := loop.NewSuppressor("edge-002", ruleSet(rules.SyncRule{Enable: true, Direction: rules.DirectionBidirectional}), fakeApplyLog{"remote-new": true})
	change := cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config", Operation: cdc.OperationUpdate,
		Before: map[string]any{"last_event_id": "remote-old", "updated_by_node": "edge-001"},
		After:  map[string]any{"last_event_id": "remote-new", "updated_by_node": "edge-001"}}
	if mustDecision(t, s, change).Upload {
		t.Fatal("remote marker transition was not suppressed")
	}
	for _, missing := range []string{"last_event_id", "updated_by_node"} {
		before := change.Before[missing]
		delete(change.Before, missing)
		decision, err := s.ShouldUpload(context.Background(), change)
		if err == nil || decision.Upload {
			t.Fatalf("incomplete before image accepted: %+v %v", decision, err)
		}
		change.Before[missing] = before
	}
}

func TestShouldUploadUsesLocalNodeScopedRule(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{
			DatabaseName:  "scada_edge",
			TableName:     "data_all",
			SourceNodeIDs: []string{"edge-002"},
			Enable:        true,
			Direction:     rules.DirectionEdgeToServer,
			PrimaryKeys:   []string{"id"},
		},
	}}
	s := loop.NewSuppressor("edge-002", set, fakeApplyLog{})

	decision := mustDecision(t, s, cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "data_all"})

	if !decision.Upload {
		t.Fatalf("expected node-scoped data_all change to upload, got %+v", decision)
	}
}

func TestShouldUploadWhenApplyLogNil(t *testing.T) {
	s := loop.NewSuppressor("edge-002", ruleSet(rules.SyncRule{
		Enable:    true,
		Direction: rules.DirectionBidirectional,
	}), nil)

	decision, err := s.ShouldUpload(context.Background(), cdc.ChangeEvent{
		DatabaseName: "scada_edge",
		TableName:    "device_config",
		After: map[string]any{
			"last_event_id":   "evt-001",
			"updated_by_node": "edge-001",
		},
	})

	if err == nil || decision.Upload {
		t.Fatalf("missing replay dependency was not rejected: decision=%+v err=%v", decision, err)
	}
}

func TestShouldUploadConvertsNonStringEventID(t *testing.T) {
	s := loop.NewSuppressor("edge-002", ruleSet(rules.SyncRule{
		Enable:    true,
		Direction: rules.DirectionBidirectional,
	}), fakeApplyLog{"12345": true})

	decision := mustDecision(t, s, cdc.ChangeEvent{
		DatabaseName: "scada_edge",
		TableName:    "device_config",
		After: map[string]any{
			"last_event_id":   12345,
			"updated_by_node": "edge-001",
		},
	})

	if decision.Upload {
		t.Fatalf("expected converted event id to suppress replay, got %+v", decision)
	}
}

func TestShouldUploadSchemaChangeRequiresRulePermission(t *testing.T) {
	change := cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config", Operation: cdc.OperationAddColumn,
		SchemaChange: &dbgovernance.SchemaChange{Operation: "ADD_COLUMN", Column: dbgovernance.ColumnDefinition{Name: "note", Type: "text", Nullable: true}}}
	blocked := mustDecision(t, loop.NewSuppressor("edge-001", ruleSet(rules.SyncRule{Enable: true, Direction: rules.DirectionBidirectional}), nil), change)
	if blocked.Upload {
		t.Fatalf("disabled schema change uploaded: %+v", blocked)
	}
	allowed := mustDecision(t, loop.NewSuppressor("edge-001", ruleSet(rules.SyncRule{Enable: true, Direction: rules.DirectionBidirectional, SchemaSync: rules.SchemaSync{AddColumns: true}}), nil), change)
	if !allowed.Upload {
		t.Fatalf("enabled schema change was suppressed: %+v", allowed)
	}
}

func TestShouldUploadDropColumnRequiresSingleSourceNode(t *testing.T) {
	change := cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config", Operation: cdc.OperationDropColumn,
		SchemaChange: &dbgovernance.SchemaChange{Operation: "DROP_COLUMN", Column: dbgovernance.ColumnDefinition{Name: "note"}}}
	rule := rules.SyncRule{Enable: true, Direction: rules.DirectionBidirectional, SchemaSync: rules.SchemaSync{DropColumns: true}}
	blocked := mustDecision(t, loop.NewSuppressor("edge-001", ruleSet(rule), nil), change)
	if blocked.Upload {
		t.Fatalf("unscoped DROP COLUMN uploaded: %+v", blocked)
	}
	rule.SourceNodeIDs = []string{"edge-001"}
	allowed := mustDecision(t, loop.NewSuppressor("edge-001", ruleSet(rule), nil), change)
	if !allowed.Upload {
		t.Fatalf("single-source DROP COLUMN was suppressed: %+v", allowed)
	}
}

func ruleSet(rule rules.SyncRule) rules.RuleSet {
	rule.DatabaseName = "scada_edge"
	rule.TableName = "device_config"
	return rules.RuleSet{Rules: []rules.SyncRule{rule}}
}

func change() cdc.ChangeEvent {
	return cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config"}
}

type fakeApplyLog map[string]bool

type fakeDeleteLog struct {
	fakeApplyLog
	deleted bool
	err     error
}

func (l fakeDeleteLog) IsDeleteReplay(_ context.Context, eventID, origin, database, table string) (bool, error) {
	if eventID != "remote-delete" || origin != "edge-001" || database != "scada_edge" || table != "device_config" {
		return false, errors.New("delete scope mismatch")
	}
	return l.deleted, l.err
}

func TestHardDeleteRequiresItsOwnReplayReceipt(t *testing.T) {
	change := cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config", Operation: cdc.OperationDelete, Before: map[string]any{"last_event_id": "remote-delete", "updated_by_node": "edge-001"}}
	set := ruleSet(rules.SyncRule{Enable: true, Direction: rules.DirectionBidirectional})
	for _, deleted := range []bool{false, true} {
		s := loop.NewSuppressor("edge-002", set, fakeDeleteLog{fakeApplyLog: fakeApplyLog{"remote-delete": true}, deleted: deleted})
		decision := mustDecision(t, s, change)
		if decision.Upload == deleted {
			t.Fatalf("delete receipt=%t decision=%+v", deleted, decision)
		}
	}
	for _, log := range []loop.ApplyLog{fakeApplyLog{"remote-delete": true}, fakeDeleteLog{err: errors.New("lookup failed")}} {
		s := loop.NewSuppressor("edge-002", set, log)
		decision, err := s.ShouldUpload(context.Background(), change)
		if err == nil || decision.Upload {
			t.Fatalf("unverified delete source accepted: %+v %v", decision, err)
		}
	}
}

func (l fakeApplyLog) Exists(_ context.Context, eventID string) (bool, error) {
	return l[eventID], nil
}

func mustDecision(t *testing.T, s *loop.Suppressor, change cdc.ChangeEvent) loop.Decision {
	t.Helper()
	decision, err := s.ShouldUpload(context.Background(), change)
	if err != nil {
		t.Fatal(err)
	}
	return decision
}

type failingApplyLog struct{ err error }

func (l failingApplyLog) Exists(ctx context.Context, _ string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return false, l.err
}

func TestReplayLookupFailureIsNotLocalBusinessChange(t *testing.T) {
	want := errors.New("replay database unavailable")
	s := loop.NewSuppressor("edge-002", ruleSet(rules.SyncRule{Enable: true, Direction: rules.DirectionBidirectional}), failingApplyLog{want})
	change := cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config", After: map[string]any{"last_event_id": "evt-1", "updated_by_node": "edge-001"}}
	decision, err := s.ShouldUpload(context.Background(), change)
	if !errors.Is(err, want) || decision.Upload {
		t.Fatalf("lookup failure must stop processing: %+v %v", decision, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	decision, err = s.ShouldUpload(ctx, change)
	if !errors.Is(err, context.Canceled) || decision.Upload {
		t.Fatalf("cancellation was not propagated: %+v %v", decision, err)
	}
}
