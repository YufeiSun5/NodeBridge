package alignment

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func generationFixture(t *testing.T, server bool) RebaselinePlan {
	t.Helper()
	rule, left, right := fixture()
	p := RebaselinePlan{MigrationID: "test-migration", EdgeNode: left.NodeID, ServerNode: right.NodeID, NodeID: left.NodeID, Mode: "edge", MySQLUUID: "owned-uuid", OldDatabase: "old_meta", Rules: []RebaselineRule{{rule}}, Tables: []RebaselineTable{{Schema: left.Schema}}}
	if server {
		p.NodeID, p.Mode = right.NodeID, "server"
		p.Tables = []RebaselineTable{{Schema: right.Schema, BackupTable: "nb_backup_" + hash([]string{right.Schema.Database, right.Schema.Table})[:32]}}
	}
	p.NewDatabase = generationDatabase(p.MigrationID, p.NodeID)
	p.NewCDCFilter = rebaselineFilter(p.Tables)
	p.ID = hash(p)
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	return p
}

func resealGeneration(p RebaselinePlan) RebaselinePlan { p.ID = ""; p.ID = hash(p); return p }

func TestRebaselinePlansBindMappingAndGeneration(t *testing.T) {
	left, right := generationFixture(t, false), generationFixture(t, true)
	if err := validateGenerationPeer(left, right, "edge", "server", "r"); err != nil {
		t.Fatal(err)
	}
	right.Rules = append([]RebaselineRule(nil), right.Rules...)
	right.Rules[0].Rule.Name = "中文显示名称"
	right = resealGeneration(right)
	if err := validateGenerationPeer(left, right, "edge", "server", "r"); err != nil {
		t.Fatal("display-only change", err)
	}
	right.Rules[0].Rule.ID = "replacement-rule"
	right = resealGeneration(right)
	if err := validateGenerationPeer(left, right, "edge", "server", "r"); err == nil {
		t.Fatal("different mappings accepted")
	}
	left.Tables[0].Schema.Table = "other_table"
	if err := resealGeneration(left).Validate(); err == nil {
		t.Fatal("forged local scope accepted")
	}
	right = generationFixture(t, true)
	right.Tables[0].BackupTable = "other_table"
	if err := resealGeneration(right).Validate(); err == nil {
		t.Fatal("arbitrary backup name accepted")
	}
}

func TestRebaselineRulesDisableAndRetainDesiredReplacement(t *testing.T) {
	r, _, _ := fixture()
	r.Enable = true
	r.ID = "new-rule"
	r.TargetDatabaseName = "new_business"
	r.Direction = rules.DirectionBidirectional
	r.ConflictPolicy = rules.ConflictLastWriteWin
	req := RebaselineRequest{MigrationID: "new", EdgeNode: "edge", ServerNode: "server", Rules: []rules.SyncRule{r}}
	result, err := normalizedRebaselineRules(req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result[0].Rule.Enable || result[0].Rule.ID != r.ID || result[0].Rule.TargetDatabaseName != "new_business" || !r.Enable {
		t.Fatal("replacement lost or input mutated")
	}
	req.Rules[0].SourceNodeIDs = []string{"other"}
	if _, err := normalizedRebaselineRules(req, nil); err == nil {
		t.Fatal("foreign member accepted")
	}
}

func TestRebaselinePendingPreparationBlocksAndRequiresConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := CheckRebaselinePreparation(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".rebaseline-pending.json", []byte("pending"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CheckRebaselinePreparation(path); err == nil {
		t.Fatal("partial publication starts")
	}
	_, err := PrepareRebaseline(context.Background(), path, "", RebaselineApply{Plan: generationFixture(t, true)})
	if err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatal(err)
	}
}

func TestRebaselineAllRulesMustCompleteBeforeStartup(t *testing.T) {
	proof, _, _ := cutoverFixture(t)
	for _, complete := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		p := generationFixture(t, false)
		p.Rules = []RebaselineRule{{proof.Rule}}
		p = resealGeneration(p)
		raw, _ := json.Marshal(p)
		mock.ExpectQuery("SELECT plan_json,prepared").WillReturnRows(sqlmock.NewRows([]string{"plan_json", "prepared"}).AddRow(raw, true))
		mock.ExpectQuery("SELECT proof_json FROM sync_rebaseline_retired").WillReturnRows(sqlmock.NewRows([]string{"proof_json"}))
		f := &CutoverFilter{DB: db, NodeID: p.NodeID}
		if complete {
			f.Proofs = []CutoverProof{proof}
		}
		err = LoadGenerationFilter(context.Background(), f)
		if (err == nil) != complete {
			t.Fatalf("complete=%v err=%v", complete, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestRebaselineRetiredEpochRequiresOriginalScopeAndIdentity(t *testing.T) {
	p, _, _ := cutoverFixture(t)
	f := &CutoverFilter{NodeID: p.Target.OriginNodeID, Retired: []CutoverProof{p}}
	e := event.SyncEvent{EventID: "retired-event", OriginNodeID: p.Source.OriginNodeID, SourceNodeID: p.Source.OriginNodeID, TargetNodeID: f.NodeID, DatabaseName: p.Source.DatabaseName, TableName: p.Source.TableName, Headers: map[string]string{EpochHeader: p.ID, SourceUUIDHeader: p.Source.MySQLServerUUID}}
	if proof, skip, err := f.Superseded(e); err != nil || !skip || proof.ID != p.ID {
		t.Fatal("old mapping not retired", err)
	}
	e.TableName = "foreign"
	if _, skip, err := f.Superseded(e); err == nil || skip {
		t.Fatal("foreign scope archived")
	}
	e.TableName = p.Source.TableName
	e.Headers[SourceUUIDHeader] = "other-server"
	if _, skip, err := f.Superseded(e); err == nil || skip {
		t.Fatal("foreign lineage archived")
	}
	e.Headers[EpochHeader] = "unknown"
	if _, skip, _ := f.Superseded(e); skip {
		t.Fatal("unknown generation archived")
	}
}
