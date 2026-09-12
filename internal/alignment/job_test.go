package alignment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func TestPendingJobGate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		job     string
		blocked bool
	}{
		{"none", sql.ErrNoRows, "", false},
		{"legacy_no_table", &mysql.MySQLError{Number: 1146}, "", false},
		{"pending", nil, strings.Repeat("a", 64), true},
		{"access_denied", &mysql.MySQLError{Number: 1142}, "", true},
		{"unavailable", errors.New("connection closed"), "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			q := mock.ExpectQuery(regexp.QuoteMeta("SELECT job_id FROM sync_alignment_job LIMIT 1"))
			if tc.err != nil {
				q.WillReturnError(tc.err)
			} else {
				q.WillReturnRows(sqlmock.NewRows([]string{"job_id"}).AddRow(tc.job))
			}
			err = CheckPendingJobs(context.Background(), db)
			if (err != nil) != tc.blocked {
				t.Fatal(err)
			}
			if tc.job != "" && !strings.Contains(err.Error(), "alignment_cutover_pending") {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestJobScopeDoesNotIncludeChangeableNodeOrPlanID(t *testing.T) {
	_, left, right := fixture()
	p := Plan{ID: strings.Repeat("a", 64), Source: left, Target: right}
	id, scope, role, err := jobIdentity(p, left.NodeID)
	if err != nil || role != "SOURCE" {
		t.Fatal(err)
	}
	p.ID, p.Source.NodeID = strings.Repeat("b", 64), "renamed"
	nextID, nextScope, _, err := jobIdentity(p, "renamed")
	if err != nil || id == nextID || scope != nextScope {
		t.Fatal("renamed node bypasses table scope fence", err)
	}
	if _, _, _, err := jobIdentity(p, "unrelated"); err == nil {
		t.Fatal("unrelated node accepted")
	}
}

func TestJobScopeUsesActualMySQLCaseMode(t *testing.T) {
	for _, mode := range []int{0, 1, 2} {
		a := canonicalJobScope("Business", "Items", mode)
		b := canonicalJobScope("business", "items", mode)
		if (a == b) != (mode != 0) {
			t.Fatalf("case mode %d mismatch", mode)
		}
	}
}

func TestReadSnapshotJobRejectsCorruptCaptureAndReceipt(t *testing.T) {
	for _, mode := range []string{"valid_plain", "valid_capture", "missing_boundary", "changed_server", "wrong_marker", "wrong_table", "negative_count", "bad_digest", "bad_phase", "bad_role"} {
		t.Run(mode, func(t *testing.T) {
			rule, left, right := fixture()
			left.HasRows = true
			plan, err := BuildPlan(rule, left, right, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			id, scope, role, err := jobIdentity(plan, left.NodeID)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			phase, count, digest := JobSourceReady, int64(2), strings.Repeat("b", 64)
			var marker any
			var capture []byte
			var server string
			boundary := boundaryFixture()
			boundary.OriginNodeID, boundary.DatabaseName, boundary.TableName = left.NodeID, left.Schema.Database, left.Schema.Table
			if mode == "valid_capture" || mode == "changed_server" || mode == "wrong_marker" || mode == "wrong_table" {
				marker = boundary.Token
				server = boundary.MySQLServerUUID
				if mode == "changed_server" {
					server = "ffffffff-ffff-ffff-ffff-ffffffffffff"
				}
				if mode == "wrong_marker" {
					boundary.Token = strings.Repeat("c", 64)
					server = ""
				}
				if mode == "wrong_table" {
					boundary.TableName = "other"
					server = ""
				}
				capture, err = json.Marshal(boundary)
				if err != nil {
					t.Fatal(err)
				}
			}
			switch mode {
			case "missing_boundary":
				marker = boundary.Token
			case "negative_count":
				count = -1
			case "bad_digest":
				digest = "not-a-digest"
			case "bad_phase":
				phase = "COMPLETE"
			case "bad_role":
				role = "TARGET"
			}
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT @@lower_case_table_names").WillReturnRows(sqlmock.NewRows([]string{"folding"}).AddRow(0))
			mock.ExpectQuery("SELECT scope_hash,plan_id.*WHERE job_id").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"scope", "plan_id", "node", "role", "phase", "plan", "count", "digest", "marker", "boundary"}).AddRow(scope, plan.ID, left.NodeID, role, phase, encoded, count, digest, marker, capture))
			if server != "" {
				mock.ExpectQuery("SELECT @@server_uuid").WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow(server))
			}
			_, err = ReadSnapshotJob(context.Background(), db, plan, left.NodeID)
			valid := mode == "valid_plain" || mode == "valid_capture"
			if (err == nil) != valid {
				t.Fatal("job validation", mode, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
