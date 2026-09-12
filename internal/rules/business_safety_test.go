package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestBusinessRuleValidation(t *testing.T) {
	base := rules.SyncRule{ID: "nb_test", DatabaseName: "nb_source", TableName: "rows", PrimaryKeys: []string{"id"}, Direction: rules.DirectionEdgeToServer, ConflictPolicy: rules.ConflictNone, Enable: true}
	for _, change := range []func(*rules.SyncRule){
		func(r *rules.SyncRule) { r.Direction = "typo" },
		func(r *rules.SyncRule) { r.ConflictPolicy = "typo" },
		func(r *rules.SyncRule) { r.PrimaryKeys = nil },
		func(r *rules.SyncRule) { r.DeleteMode = "TRUNCATE" },
		func(r *rules.SyncRule) { r.Direction = rules.DirectionBidirectional },
		func(r *rules.SyncRule) { r.ConflictPolicy = rules.ConflictServerWin },
		func(r *rules.SyncRule) { r.ConflictPolicy = rules.ConflictLastWriteWin },
	} {
		r := base
		change(&r)
		if err := (rules.RuleSet{Rules: []rules.SyncRule{r}}).Validate(); err == nil {
			t.Fatalf("invalid rule accepted: %+v", r)
		}
	}
	for _, mode := range []string{"", rules.DeleteSoft, rules.DeleteHard} {
		r := base
		r.DeleteMode = mode
		if err := (rules.RuleSet{Rules: []rules.SyncRule{r}}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLegacyDeleteModeRoundTripIsExplicitSoft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.yaml")
	legacy := "rules:\n  - id: test\n    database_name: nb_test\n    table_name: rows\n    primary_keys: [id]\n    direction: EDGE_TO_SERVER\n    conflict_policy: NONE\n    enable: true\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	set, err := rules.LoadFile(path)
	if err != nil || set.Rules[0].DeleteMode != rules.DeleteSoft {
		t.Fatalf("legacy read: %+v %v", set, err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != legacy {
		t.Fatal("reading a legacy rule must not modify its file")
	}
	if err := rules.SaveFile(path, *set); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "delete_mode: SOFT") {
		t.Fatal("save must make legacy deletion semantics explicit")
	}
}
