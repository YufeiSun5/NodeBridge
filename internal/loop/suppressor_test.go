package loop_test

import (
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestShouldUploadWhenRuleMissing(t *testing.T) {
	s := loop.NewSuppressor("edge-001", rules.RuleSet{}, fakeApplyLog{})

	decision := s.ShouldUpload(cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config"})

	if decision.Upload {
		t.Fatalf("expected no upload, got %+v", decision)
	}
	if decision.Reason != "table not in sync rules" {
		t.Fatalf("unexpected reason %q", decision.Reason)
	}
}

func TestShouldUploadWhenRuleDisabled(t *testing.T) {
	s := loop.NewSuppressor("edge-001", ruleSet(rules.SyncRule{Enable: false}), fakeApplyLog{})

	decision := s.ShouldUpload(change())

	if decision.Upload {
		t.Fatalf("expected disabled rule to suppress upload, got %+v", decision)
	}
}

func TestShouldUploadWhenIgnored(t *testing.T) {
	s := loop.NewSuppressor("edge-001", ruleSet(rules.SyncRule{
		Enable:    true,
		Direction: rules.DirectionIgnore,
	}), fakeApplyLog{})

	decision := s.ShouldUpload(change())

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

	decision := s.ShouldUpload(cdc.ChangeEvent{
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

func TestShouldUploadLocalBusinessChange(t *testing.T) {
	s := loop.NewSuppressor("edge-001", ruleSet(rules.SyncRule{
		Enable:    true,
		Direction: rules.DirectionBidirectional,
	}), fakeApplyLog{"evt-001": true})

	decision := s.ShouldUpload(cdc.ChangeEvent{
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

func TestShouldUploadWhenApplyLogNil(t *testing.T) {
	s := loop.NewSuppressor("edge-002", ruleSet(rules.SyncRule{
		Enable:    true,
		Direction: rules.DirectionBidirectional,
	}), nil)

	decision := s.ShouldUpload(cdc.ChangeEvent{
		DatabaseName: "scada_edge",
		TableName:    "device_config",
		After: map[string]any{
			"last_event_id":   "evt-001",
			"updated_by_node": "edge-001",
		},
	})

	if !decision.Upload {
		t.Fatalf("expected upload without apply log evidence, got %+v", decision)
	}
}

func TestShouldUploadConvertsNonStringEventID(t *testing.T) {
	s := loop.NewSuppressor("edge-002", ruleSet(rules.SyncRule{
		Enable:    true,
		Direction: rules.DirectionBidirectional,
	}), fakeApplyLog{"12345": true})

	decision := s.ShouldUpload(cdc.ChangeEvent{
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

func ruleSet(rule rules.SyncRule) rules.RuleSet {
	rule.DatabaseName = "scada_edge"
	rule.TableName = "device_config"
	return rules.RuleSet{Rules: []rules.SyncRule{rule}}
}

func change() cdc.ChangeEvent {
	return cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config"}
}

type fakeApplyLog map[string]bool

func (l fakeApplyLog) Exists(eventID string) bool {
	return l[eventID]
}
