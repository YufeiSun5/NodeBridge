package datasyncui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/dbgovernance"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

func TestMCPTrustedProcessTimesSurviveNumericSecret(t *testing.T) {
	cfg := validConfig()
	cfg.MySQL.Password = "2026"
	service := &MCPService{}
	service.Config = cfg
	before := uiapi.AgentProcessStatus{PID: 20261, Status: "running", StartedAt: "2026-09-11T15:00:00+08:00", ExitedAt: "2026-09-11T15:01:00+08:00", LastError: "credential 2026 failed"}
	after := service.redact(before).(uiapi.AgentProcessStatus)
	if after.StartedAt != before.StartedAt || after.ExitedAt != before.ExitedAt || after.PID != before.PID {
		t.Fatal("trusted process metadata changed", after)
	}
	if strings.Contains(after.LastError, cfg.MySQL.Password) {
		t.Fatal("free-text secret was not redacted")
	}
}

func TestMCPRuleRevisionCrossClientProtection(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	rulesPath := filepath.Join(dir, "rules.yaml")
	one, err := NewMCPService(configPath, rulesPath, "", true)
	if err != nil {
		t.Fatal(err)
	}
	two, err := NewMCPService(configPath, rulesPath, "", true)
	if err != nil {
		t.Fatal(err)
	}
	set := []rules.SyncRule{{ID: "owned", DatabaseName: "owned", TableName: "items", PrimaryKeys: []string{"id"}, DeleteMode: rules.DeleteHard}}
	args, _ := json.Marshal(map[string]any{"rules": set})
	if out := callMCP(t, one, "nodebridge_save_sync_rules", string(args)); strings.Contains(out, `"isError":true`) {
		t.Fatal(out)
	}
	revision, err := rules.FileRevision(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	if out := callMCP(t, two, "nodebridge_sync_rules", `{}`); !strings.Contains(out, revision) {
		t.Fatal(out)
	}
	set[0].ID = "writer-one"
	args, _ = json.Marshal(map[string]any{"rules": set, "expected_revision": revision})
	if out := callMCP(t, one, "nodebridge_save_sync_rules", string(args)); strings.Contains(out, `"isError":true`) {
		t.Fatal(out)
	}
	set[0].ID = "writer-two"
	args, _ = json.Marshal(map[string]any{"rules": set, "expected_revision": revision})
	if out := callMCP(t, two, "nodebridge_save_sync_rules", string(args)); !strings.Contains(out, "revision_conflict") {
		t.Fatal(out)
	}
	loaded, err := rules.LoadFile(rulesPath)
	if err != nil || loaded.Rules[0].ID != "writer-one" {
		t.Fatalf("lost update: %+v %v", loaded, err)
	}
}

func TestRuleDatabaseSelectorCannotEscapeReferencedTable(t *testing.T) {
	s, err := NewMCPService(filepath.Join(t.TempDir(), "config.yaml"), "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	s.app.ruleSet = &rules.RuleSet{Rules: []rules.SyncRule{{ID: "owned", DatabaseName: "source_db", TableName: "source_items", TargetDatabaseName: "target_db", TargetTableName: "target_items"}}}
	for _, tc := range []struct {
		scope       RuleDatabaseScope
		table, want string
	}{
		{RuleDatabaseScope{RuleID: "owned", Side: "target"}, "target_items", "target_db"},
		{RuleDatabaseScope{RuleID: "owned", Side: "source"}, "source_items", "source_db"},
		{RuleDatabaseScope{RuleID: "owned", Side: "target"}, "unrelated", ""},
		{RuleDatabaseScope{RuleID: "unknown", Side: "target"}, "target_items", ""},
		{RuleDatabaseScope{RuleID: "owned", Side: "invalid"}, "target_items", ""},
	} {
		got, err := s.ruleDatabase(tc.scope, tc.table)
		if tc.want == "" && err == nil || tc.want != "" && (err != nil || got != tc.want) {
			t.Fatalf("got=%s err=%v", got, err)
		}
	}
	_, err = s.mysqlGovernanceQuery(RuleQueryRequest{RuleDatabaseScope: RuleDatabaseScope{RuleID: "unknown", Side: "target"}, QueryRequest: dbgovernance.QueryRequest{Table: "target_items"}})
	if err == nil || !strings.Contains(err.Error(), "unknown rule_id") {
		t.Fatalf("unknown selector reached database: %v", err)
	}
}

func TestRulesDTOReportsActiveAndSavedSeparately(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.yaml")
	path := filepath.Join(dir, "rules.yaml")
	release, err := agentstate.Acquire(config, filepath.Join(dir, "stop"))
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := agentstate.PublishRules(config, path, "running-revision"); err != nil {
		t.Fatal(err)
	}
	app := &App{configPath: config, rulesPath: path}
	active := app.rulesDTO(nil, "running-revision")
	if active.Activation != "active" || active.ActiveRevision != "running-revision" {
		t.Fatal(active)
	}
	changed := app.rulesDTO(nil, "new-saved-revision")
	if changed.Activation != "restart_required" || changed.ActiveRevision != "running-revision" || changed.SavedRevision != "new-saved-revision" {
		t.Fatal(changed)
	}
}

func TestEdgeRuleDatabaseUsesRuntimeDownlinkMapping(t *testing.T) {
	s, err := NewMCPService(filepath.Join(t.TempDir(), "config.yaml"), "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	s.Config.Mode = appconfig.ModeEdge
	s.Config.MySQL.Database = "local_default"
	s.app.ruleSet = &rules.RuleSet{Rules: []rules.SyncRule{{ID: "owned", DatabaseName: "source_db", TableName: "source_rows", TargetDatabaseName: "explicit_target", TargetTableName: "target_rows", Direction: rules.DirectionServerToEdge}}}
	for _, tc := range []struct{ direction, want string }{{rules.DirectionServerToEdge, "explicit_target"}, {rules.DirectionEdgeToServer, "local_default"}} {
		s.app.ruleSet.Rules[0].Direction = tc.direction
		got, err := s.ruleDatabase(RuleDatabaseScope{RuleID: "owned", Side: "target"}, "target_rows")
		if err != nil || got != tc.want {
			t.Fatalf("direction=%s database=%s err=%v", tc.direction, got, err)
		}
	}
}

func TestRulePreflightRejectsUnknownRuleBeforeConnection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")
	if err := rules.SaveFile(path, rules.RuleSet{Rules: []rules.SyncRule{}}); err != nil {
		t.Fatal(err)
	}
	app := &App{rulesPath: path}
	if _, err := app.PreflightSyncRule(RulePreflightRequest{RuleID: "not-owned", Side: "target"}); err == nil || !strings.Contains(err.Error(), "unknown rule_id") {
		t.Fatalf("result: %v", err)
	}
}
