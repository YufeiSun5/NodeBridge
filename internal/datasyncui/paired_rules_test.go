package datasyncui

import (
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestBindReadyRulesRequiresMatchingLocalProof(t *testing.T) {
	rule := rules.SyncRule{ID: "paired", DatabaseName: "business", TableName: "items", PrimaryKeys: []string{"id"}, Enable: true, Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin}
	proof := alignment.CutoverProof{Rule: rule, Source: alignment.SnapshotBoundary{OriginNodeID: "edge"}, Target: alignment.SnapshotBoundary{OriginNodeID: "server"}}
	for _, mode := range []string{"matching", "renamed", "missing", "wrong_node", "mapping_changed", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			set := rules.RuleSet{Rules: []rules.SyncRule{rule}}
			proofs, node := []alignment.CutoverProof{proof}, "edge"
			switch mode {
			case "renamed":
				set.Rules[0].Name = "检测标准同步"
			case "missing":
				proofs = nil
			case "wrong_node":
				node = "unrelated"
			case "mapping_changed":
				set.Rules[0].TargetTableName = "another"
			case "disabled":
				set.Rules[0].Enable = false
				proofs = nil
			}
			err := bindReadyRules(&set, proofs, node)
			wantOK := mode == "matching" || mode == "renamed" || mode == "disabled"
			if (err == nil) != wantOK {
				t.Fatal(mode, err)
			}
			if wantOK {
				if err := set.Validate(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
