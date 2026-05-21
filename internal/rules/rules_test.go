package rules_test

import (
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestRuleSetFind(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{ID: "edge-device", DatabaseName: "scada_edge", TableName: "device_config"},
		{ID: "center-device", DatabaseName: "scada_center", TableName: "device_config"},
	}}

	rule := set.Find("scada_edge", "device_config")
	if rule == nil {
		t.Fatal("expected rule")
	}
	if rule.ID != "edge-device" {
		t.Fatalf("expected edge-device, got %q", rule.ID)
	}
}

func TestRuleSetFindDoesNotMatchDifferentDatabase(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{ID: "center-device", DatabaseName: "scada_center", TableName: "device_config"},
	}}

	if rule := set.Find("scada_edge", "device_config"); rule != nil {
		t.Fatalf("expected no rule, got %+v", rule)
	}
}

func TestRuleSetFindMissing(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{ID: "alarm", DatabaseName: "scada_edge", TableName: "alarm_history"},
	}}

	if rule := set.Find("scada_edge", "device_config"); rule != nil {
		t.Fatalf("expected no rule, got %+v", rule)
	}
}
