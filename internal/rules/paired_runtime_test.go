package rules

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDisabledHardBidirectionalDraftCanBeSaved(t *testing.T) {
	rule := SyncRule{ID: "draft", DatabaseName: "db", TableName: "rows", PrimaryKeys: []string{"id"}, Direction: DirectionBidirectional, ConflictPolicy: ConflictLastWriteWin, DeleteMode: DeleteHard, InitialAlignment: InitialAlignment{Policy: AlignmentManual}}
	path := filepath.Join(t.TempDir(), "rules.yaml")
	if err := SaveFile(path, RuleSet{Rules: []SyncRule{rule}}); err != nil {
		t.Fatal(err)
	}
	set, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if set.Rules[0].Enable || set.Rules[0].DeleteMode != DeleteHard {
		t.Fatal("draft mutated")
	}
	set.Rules[0].Enable = true
	if err := SaveFile(path, *set); err == nil {
		t.Fatal("ordinary save activated unpaired runtime")
	}
}

func TestPairedRuntimeBindingIsNotConfiguration(t *testing.T) {
	rule := SyncRule{ID: "paired", Enable: true, DatabaseName: "db", TableName: "rows", PrimaryKeys: []string{"id"}, Direction: DirectionBidirectional, ConflictPolicy: ConflictLastWriteWin, DeleteMode: DeleteHard}
	if rule.ValidateRuntimePolicy() == nil {
		t.Fatal("unbound rule allowed")
	}
	bound := rule.BindPairedRuntime()
	if err := (RuleSet{Rules: []SyncRule{bound}}).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"json", "yaml"} {
		var raw []byte
		var err error
		var roundtrip SyncRule
		if format == "json" {
			raw, err = json.Marshal(bound)
			if err == nil {
				err = json.Unmarshal(raw, &roundtrip)
			}
		} else {
			raw, err = yaml.Marshal(bound)
			if err == nil {
				err = yaml.Unmarshal(raw, &roundtrip)
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		if roundtrip.ValidateRuntimePolicy() == nil {
			t.Fatalf("runtime binding leaked into %s", format)
		}
	}
	for _, modify := range []func(*SyncRule){func(r *SyncRule) { r.ConflictPolicy = ConflictServerWin }, func(r *SyncRule) { r.Direction = DirectionEdgeToServer }, func(r *SyncRule) { r.SyncMode = SyncModeCRUDCompact }, func(r *SyncRule) { r.SchemaSync.AddColumns = true }} {
		candidate := bound
		modify(&candidate)
		if candidate.ValidateRuntimePolicy() == nil {
			t.Fatal("binding allowed an unverified policy")
		}
	}
}
