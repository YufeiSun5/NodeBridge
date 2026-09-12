package alignment

import (
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"testing"
	"time"
)

func fixture() (rules.SyncRule, Observation, Observation) {
	r := rules.SyncRule{ID: "r", DatabaseName: "left_db", TableName: "source_rows", TargetDatabaseName: "right_db", TargetTableName: "target_rows", PrimaryKeys: []string{"id"}, TargetPrimaryKeys: []string{"target_id"}, Direction: rules.DirectionEdgeToServer, DeleteMode: rules.DeleteHard, InitialAlignment: rules.InitialAlignment{Policy: rules.AlignmentManual}, ColumnMappings: []rules.ColumnMapping{{SourceColumn: "value", TargetColumn: "target_value"}}}
	l := Observation{NodeID: "edge", Schema: rulecheck.Schema{Database: r.DatabaseName, Table: r.TableName, Engine: "InnoDB", PrimaryKeys: []string{"id"}, Columns: []rulecheck.Column{{Name: "id", Type: "bigint"}, {Name: "value", Type: "varchar(20)", Collation: "utf8mb4_bin"}}}}
	right := Observation{NodeID: "server", Schema: rulecheck.Schema{Database: r.TargetDatabaseName, Table: r.TargetTableName, Engine: "InnoDB", PrimaryKeys: []string{"target_id"}, Columns: []rulecheck.Column{{Name: "target_id", Type: "bigint"}, {Name: "target_value", Type: "varchar(20)", Collation: "utf8mb4_bin"}}}}
	return r, l, right
}

func TestEmptySideChoosesDirection(t *testing.T) {
	for _, tc := range []struct {
		left, right bool
		want        string
	}{{true, false, ToRight}, {false, true, ToLeft}, {false, false, NoCopy}, {true, true, ""}} {
		r, l, right := fixture()
		l.HasRows, right.HasRows = tc.left, tc.right
		p, err := BuildPlan(r, l, right, time.Now())
		if tc.want == "" {
			if err == nil {
				t.Fatal("both nonempty accepted")
			}
			continue
		}
		if err != nil || p.Direction != tc.want {
			t.Fatalf("%+v %v", p, err)
		}
		if err := p.Validate(r, p.CreatedAt, true); err != nil {
			t.Fatal(err)
		}
		if tc.want == ToLeft && (p.Columns[0].Source != "target_id" || p.Columns[0].Target != "id") {
			t.Fatalf("reverse PK mapping lost: %+v", p.Columns)
		}
	}
}
func TestPlanRejectsUnconfirmedStaleChangedAndAutomatic(t *testing.T) {
	r, l, right := fixture()
	l.HasRows = true
	p, err := BuildPlan(r, l, right, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if p.Validate(r, p.CreatedAt, false) == nil {
		t.Fatal("missing confirmation accepted")
	}
	if p.Validate(r, p.ExpiresAt, true) == nil {
		t.Fatal("expired accepted")
	}
	changed := r
	changed.TableName = "other"
	if p.Validate(changed, p.CreatedAt, true) == nil {
		t.Fatal("changed rule accepted")
	}
	q := p
	q.Target.NodeID = "other"
	if q.Validate(r, p.CreatedAt, true) == nil {
		t.Fatal("changed target accepted")
	}
	r.InitialAlignment.Policy = rules.AlignmentDisabled
	if _, err := BuildPlan(r, l, right, time.Now()); err == nil {
		t.Fatal("disabled accepted")
	}
}
func TestRejectLossyOrAmbiguousMapping(t *testing.T) {
	for _, change := range []func(*rules.SyncRule, *Observation, *Observation){
		func(r *rules.SyncRule, l, rr *Observation) {
			r.ColumnMappings = append(r.ColumnMappings, rules.ColumnMapping{SourceColumn: "id", TargetColumn: "target_value"})
			r.TargetPrimaryKeys = nil
		},
		func(r *rules.SyncRule, l, rr *Observation) { rr.Schema.Columns[1].Type = "varchar(10)" },
		func(r *rules.SyncRule, l, rr *Observation) { rr.Schema.Columns[1].Collation = "utf8mb4_general_ci" },
		func(r *rules.SyncRule, l, rr *Observation) { r.ExcludeColumns = []string{"id"} },
		func(r *rules.SyncRule, l, rr *Observation) { rr.NodeID = l.NodeID },
		func(r *rules.SyncRule, l, rr *Observation) { rr.Schema.Table = "other" },
		func(r *rules.SyncRule, l, rr *Observation) {
			rr.Schema.Columns = append(rr.Schema.Columns, rulecheck.Column{Name: "required_local", Type: "int"})
		},
	} {
		r, l, rr := fixture()
		l.HasRows = true
		change(&r, &l, &rr)
		if _, err := BuildPlan(r, l, rr, time.Now()); err == nil {
			t.Fatalf("unsafe mapping accepted: %+v", r)
		}
	}
}

func TestBidirectionalPlanRequiresCompleteReversibleRow(t *testing.T) {
	r, l, right := fixture()
	r.Direction, r.ConflictPolicy, r.DeleteMode = rules.DirectionBidirectional, rules.ConflictLastWriteWin, rules.DeleteSoft
	metadata := []rulecheck.Column{{Name: "last_event_id", Type: "varchar(128)"}, {Name: "updated_by_node", Type: "varchar(64)"}, {Name: "is_deleted", Type: "tinyint"}, {Name: "deleted_at", Type: "datetime(6)", Nullable: true}, {Name: "deleted_by_node", Type: "varchar(64)", Nullable: true}}
	l.Schema.Columns = append(l.Schema.Columns, metadata...)
	right.Schema.Columns = append(right.Schema.Columns, metadata...)
	if _, err := BuildPlan(r, l, right, time.Now()); err != nil {
		t.Fatal(err)
	}
	right.Schema.Columns = append(right.Schema.Columns, rulecheck.Column{Name: "local_only", Type: "int", Nullable: true})
	if _, err := BuildPlan(r, l, right, time.Now()); err == nil {
		t.Fatal("partial bidirectional state accepted")
	}
}
