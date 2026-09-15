package alignment

import (
	"testing"
	"time"
)

func TestRuleNamePreservesLegacyPlanAndProof(t *testing.T) {
	rule, left, right := fixture()
	now := time.Now()
	plan, err := BuildPlan(rule, left, right, now)
	if err != nil {
		t.Fatal(err)
	}
	proof := CutoverProof{Rule: rule}
	rule.Name = "检测标准同步"
	if err := plan.Validate(rule, now, true); err != nil {
		t.Fatal(err)
	}
	next, err := BuildPlan(rule, left, right, now)
	if err != nil || next.ID != plan.ID || !proof.MatchesRule(rule) {
		t.Fatal(next, err)
	}
	rule.ID = "different-id"
	if plan.Validate(rule, now, true) == nil || proof.MatchesRule(rule) {
		t.Fatal("name exemption admitted an ID change")
	}
}
