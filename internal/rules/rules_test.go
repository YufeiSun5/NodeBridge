package rules_test

import (
	"os"
	"path/filepath"
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

func TestLoadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.yaml")
	if err := os.WriteFile(path, []byte(`
rules:
  - id: device
    database_name: scada_edge
    table_name: device_config
    target_table_name: device_settings
    enable: true
    primary_keys: [id]
    target_primary_keys: [setting_id]
    column_mappings:
      - source_column: id
        target_column: setting_id
`), 0o600); err != nil {
		t.Fatalf("write rules: %v", err)
	}

	set, err := rules.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}
	rule := set.Find("scada_edge", "device_config")
	if rule == nil {
		t.Fatal("expected loaded rule")
	}
	if rule.TargetTableName != "device_settings" {
		t.Fatalf("unexpected target table %q", rule.TargetTableName)
	}
	if len(rule.ColumnMappings) != 1 || rule.ColumnMappings[0].TargetColumn != "setting_id" {
		t.Fatalf("unexpected column mappings %+v", rule.ColumnMappings)
	}
}
