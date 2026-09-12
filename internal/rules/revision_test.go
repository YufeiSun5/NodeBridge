package rules_test

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestRuleCASRejectsMissingAndStaleRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.yaml")
	set := rules.RuleSet{Rules: []rules.SyncRule{{ID: "r", DatabaseName: "owned", TableName: "items", PrimaryKeys: []string{"id"}, DeleteMode: rules.DeleteHard}}}
	first, err := rules.SaveFileCAS(path, set, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rules.SaveFileCAS(path, set, ""); err == nil || !strings.Contains(err.Error(), "expected_revision_required") {
		t.Fatalf("missing revision: %v", err)
	}
	set.Rules[0].ID = "changed"
	next, err := rules.SaveFileCAS(path, set, first)
	if err != nil || next == first {
		t.Fatalf("save: %s %v", next, err)
	}
	if _, err := rules.SaveFileCAS(path, set, first); err == nil || !strings.Contains(err.Error(), "revision_conflict") {
		t.Fatalf("stale revision: %v", err)
	}
	loaded, rev, err := rules.LoadFileWithRevision(path)
	if err != nil || rev != next || loaded.Rules[0].ID != "changed" {
		t.Fatalf("read: %+v %s %v", loaded, rev, err)
	}
}

func TestRuleCASConcurrentWritersHaveOneWinner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.yaml")
	set := rules.RuleSet{Rules: []rules.SyncRule{{ID: "r", DatabaseName: "owned", TableName: "items", PrimaryKeys: []string{"id"}}}}
	initial, err := rules.SaveFileCAS(path, set, "")
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan error, 8)
	for i := range 8 {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			local := rules.RuleSet{Rules: append([]rules.SyncRule(nil), set.Rules...)}
			local.Rules[0].ID = string(rune('a' + i))
			_, err := rules.SaveFileCAS(path, local, initial)
			results <- err
		}(i)
	}
	group.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !strings.Contains(err.Error(), "revision_conflict") {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("winners=%d", winners)
	}
}
