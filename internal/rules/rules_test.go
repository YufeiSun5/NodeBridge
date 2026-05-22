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

func TestRuleSetFindForNodePrefersScopedRule(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{
			ID:                 "data-all-default",
			DatabaseName:       "scada_edge",
			TableName:          "data_all",
			TargetDatabaseName: "scada_center",
			TargetTableName:    "data_all_default",
			PrimaryKeys:        []string{"id"},
		},
		{
			ID:                 "data-all-edge-002",
			DatabaseName:       "scada_edge",
			TableName:          "data_all",
			SourceNodeIDs:      []string{"edge-002"},
			TargetDatabaseName: "scada_center",
			TargetTableName:    "data_all_edge_002",
			PrimaryKeys:        []string{"id"},
		},
	}}

	rule := set.FindForNode("scada_edge", "data_all", "edge-002", "edge-002")
	if rule == nil {
		t.Fatal("expected scoped rule")
	}
	if rule.ID != "data-all-edge-002" || rule.TargetTableName != "data_all_edge_002" {
		t.Fatalf("unexpected scoped rule %+v", rule)
	}
}

func TestRuleSetFindForNodeUsesUnscopedFallback(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{
			ID:              "data-all-default",
			DatabaseName:    "scada_edge",
			TableName:       "data_all",
			PrimaryKeys:     []string{"id"},
			TargetTableName: "data_all_default",
		},
		{
			ID:            "data-all-edge-002",
			DatabaseName:  "scada_edge",
			TableName:     "data_all",
			SourceNodeIDs: []string{"edge-002"},
			PrimaryKeys:   []string{"id"},
		},
	}}

	rule := set.FindForNode("scada_edge", "data_all", "edge-009", "edge-009")
	if rule == nil {
		t.Fatal("expected fallback rule")
	}
	if rule.ID != "data-all-default" {
		t.Fatalf("expected fallback rule, got %+v", rule)
	}
}

func TestRuleSetValidateAllowsSameTableForDifferentSourceNodes(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{
			DatabaseName:  "scada_edge",
			TableName:     "data_all",
			SourceNodeIDs: []string{"edge-001"},
			PrimaryKeys:   []string{"id"},
		},
		{
			DatabaseName:  "scada_edge",
			TableName:     "data_all",
			SourceNodeIDs: []string{"edge-002"},
			PrimaryKeys:   []string{"id"},
		},
	}}
	if err := set.Validate(); err != nil {
		t.Fatalf("expected scoped same-table rules to validate: %v", err)
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

func TestDefaultRuleSetIsUsable(t *testing.T) {
	set := rules.DefaultRuleSet()
	if len(set.Rules) < 2 {
		t.Fatalf("expected MVP default rules, got %+v", set.Rules)
	}
	if err := set.Validate(); err != nil {
		t.Fatalf("DefaultRuleSet must validate: %v", err)
	}
	device := set.Find("scada_edge", "device_config")
	if device == nil || device.TargetTableName != "device_settings" {
		t.Fatalf("expected device_config remap default, got %+v", device)
	}
}

func TestTenEdgeExampleRulesValidate(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "sync-rules.10-edge.example.yaml")
	set, err := rules.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}
	if err := set.Validate(); err != nil {
		t.Fatalf("10-edge example rules must validate: %v", err)
	}
	rule := set.FindForNode("scada_edge", "data_all", "edge-010", "edge-010")
	if rule == nil {
		t.Fatal("expected edge-010 data_all rule")
	}
	if rule.TargetTableName != "data_all_edge_010" {
		t.Fatalf("unexpected target table %q", rule.TargetTableName)
	}
}

func TestRuleSetValidateRejectsDuplicateRule(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{DatabaseName: "scada_edge", TableName: "device_config", PrimaryKeys: []string{"id"}},
		{DatabaseName: "scada_edge", TableName: "device_config", PrimaryKeys: []string{"id"}},
	}}
	if err := set.Validate(); err == nil {
		t.Fatal("expected duplicate rule error")
	}
}

func TestRuleSetValidateRejectsDuplicateScopedRule(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{DatabaseName: "scada_edge", TableName: "data_all", SourceNodeIDs: []string{"edge-001"}, PrimaryKeys: []string{"id"}},
		{DatabaseName: "scada_edge", TableName: "data_all", SourceNodeIDs: []string{"edge-001"}, PrimaryKeys: []string{"id"}},
	}}
	if err := set.Validate(); err == nil {
		t.Fatal("expected duplicate scoped rule error")
	}
}

func TestRuleSetValidateRejectsInvalidIdentifier(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{DatabaseName: "scada_edge", TableName: "device-config", PrimaryKeys: []string{"id"}},
	}}
	if err := set.Validate(); err == nil {
		t.Fatal("expected invalid identifier error")
	}
}

func TestRuleSetValidateRejectsInvalidSourceNodeID(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{DatabaseName: "scada_edge", TableName: "data_all", SourceNodeIDs: []string{"edge 001"}, PrimaryKeys: []string{"id"}},
	}}
	if err := set.Validate(); err == nil {
		t.Fatal("expected invalid source node id error")
	}
}

func TestRuleSetValidateRejectsInvalidDispatchTarget(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{
			DatabaseName:   "scada_edge",
			TableName:      "device_config",
			Direction:      rules.DirectionBidirectional,
			DispatchTarget: "ALL_THE_THINGS",
			PrimaryKeys:    []string{"id"},
		},
	}}
	if err := set.Validate(); err == nil {
		t.Fatal("expected invalid dispatch target error")
	}
}

func TestRuleSetValidateRequiresSelectedDispatchNodes(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{
			DatabaseName:   "scada_edge",
			TableName:      "device_config",
			Direction:      rules.DirectionBidirectional,
			DispatchTarget: rules.DispatchSelectedEdges,
			PrimaryKeys:    []string{"id"},
		},
	}}
	if err := set.Validate(); err == nil {
		t.Fatal("expected selected dispatch node validation error")
	}
}

func TestRuleSetValidateRejectsMismatchedTargetPrimaryKeys(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{DatabaseName: "scada_edge", TableName: "device_config", PrimaryKeys: []string{"id"}, TargetPrimaryKeys: []string{"id", "extra"}},
	}}
	if err := set.Validate(); err == nil {
		t.Fatal("expected primary key mismatch error")
	}
}

func TestRuleSetValidateRejectsDuplicateTargetColumnMapping(t *testing.T) {
	set := rules.RuleSet{Rules: []rules.SyncRule{
		{
			DatabaseName: "scada_edge",
			TableName:    "device_config",
			PrimaryKeys:  []string{"id"},
			ColumnMappings: []rules.ColumnMapping{
				{SourceColumn: "name", TargetColumn: "display_name"},
				{SourceColumn: "value", TargetColumn: "display_name"},
			},
		},
	}}
	if err := set.Validate(); err == nil {
		t.Fatal("expected duplicate target column mapping error")
	}
}
