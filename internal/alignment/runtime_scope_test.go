package alignment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
)

func TestRuntimeScopesFilterBothJobsAndTopology(t *testing.T) {
	for _, pending := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectQuery("SELECT @@lower_case_table_names").WillReturnRows(sqlmock.NewRows([]string{"folding"}).AddRow(1))
		scope := canonicalJobScope("business", "notes", 1)
		mock.ExpectQuery("SELECT job_id,node_id,scope_hash,plan_json FROM sync_alignment_job.*scope_hash IN").WithArgs(scope).WillReturnRows(sqlmock.NewRows([]string{"job_id", "node_id", "scope_hash", "plan_json"}))
		rows := sqlmock.NewRows([]string{"node_id", "intent_json", "topology_json", "phase"})
		if pending {
			rows.AddRow("node", []byte(`{}`), nil, "PENDING")
		}
		mock.ExpectQuery("SELECT node_id,intent_json,topology_json,phase FROM sync_alignment_topology WHERE scope_hash IN").WithArgs(scope).WillReturnRows(rows)
		_, err = LoadActiveCutoversForTables(context.Background(), db, []TableScope{{"Business", "Notes"}, {"business", "notes"}})
		if pending {
			if err == nil || !strings.Contains(err.Error(), "alignment_topology_pending") {
				t.Fatal(err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestReleasedScopeStillLoadsActiveTableProof(t *testing.T) {
	for _, table := range []string{"notes", "other"} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectQuery("SELECT @@lower_case_table_names").WillReturnRows(sqlmock.NewRows([]string{"folding"}).AddRow(0))
		plan := Plan{Source: Observation{NodeID: "node", Schema: rulecheck.Schema{Database: "business", Table: table}}}
		data, _ := json.Marshal(plan)
		mock.ExpectQuery("SELECT job_id,node_id,scope_hash,plan_json").WithArgs(canonicalJobScope("business", "notes", 0)).WillReturnRows(sqlmock.NewRows([]string{"job_id", "node_id", "scope_hash", "plan_json"}).AddRow("job", "node", "released-reservation", data))
		if table == "notes" {
			mock.ExpectQuery("SELECT proof_id,proof_json,phase,peer_node_id").WithArgs("job").WillReturnError(errors.New("test proof lookup"))
		} else {
			mock.ExpectQuery("SELECT node_id,intent_json,topology_json,phase").WillReturnRows(sqlmock.NewRows([]string{"node_id", "intent_json", "topology_json", "phase"}))
		}
		_, err = LoadActiveCutoversForTables(context.Background(), db, []TableScope{{"business", "notes"}})
		if table == "notes" {
			if err == nil || !strings.Contains(err.Error(), "test proof lookup") {
				t.Fatal(err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestNoEnabledTablesDoesNotConsultUnrelatedJobs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	proofs, err := LoadActiveCutoversForTables(context.Background(), db, nil)
	if err != nil || len(proofs) != 0 {
		t.Fatal(proofs, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
