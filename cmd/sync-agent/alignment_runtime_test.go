package main

import (
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestEnabledBidirectionalCannotStartWithManifestAlone(t *testing.T) {
	rule := rules.SyncRule{ID: "pair", Enable: true, Direction: rules.DirectionBidirectional, DatabaseName: "business", TableName: "items"}
	proof := alignment.CutoverProof{Rule: rule, Source: alignment.SnapshotBoundary{OriginNodeID: "edge", DatabaseName: "business", TableName: "items"}}
	for _, mode := range []string{"ready", "no_proof", "wrong_node", "changed_rule", "removed_rule", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			set := rules.RuleSet{Rules: []rules.SyncRule{rule}}
			proofs, node := []alignment.CutoverProof{proof}, "edge"
			switch mode {
			case "no_proof":
				proofs = nil
			case "wrong_node":
				node = "other"
			case "changed_rule":
				set.Rules[0].TableName = "changed"
			case "removed_rule":
				set.Rules = nil
			case "disabled":
				set.Rules[0].Enable = false
			}
			err := requireRuntimeCutovers(node, &set, proofs)
			if (err == nil) != (mode == "ready" || mode == "disabled") {
				t.Fatal(mode, err)
			}
		})
	}
}

func TestDisabledCutoverDoesNotArchiveAndAckInput(t *testing.T) {
	proof := alignment.CutoverProof{Source: alignment.SnapshotBoundary{OriginNodeID: "edge", DatabaseName: "business", TableName: "items"}}
	filter := &alignment.CutoverFilter{NodeID: "edge", Proofs: []alignment.CutoverProof{proof}}
	for _, enabled := range []bool{false, true} {
		endpoint := rulecheck.EndpointRules{Capture: rules.RuleSet{Rules: []rules.SyncRule{{Enable: enabled, DatabaseName: "business", TableName: "items"}}}}
		selected := enabledCutovers(filter, endpoint)
		if (len(selected.Proofs) == 1) != enabled {
			t.Fatal("disabled input could be archived", selected)
		}
		if len(filter.Proofs) != 1 {
			t.Fatal("caller proofs mutated")
		}
	}
}
