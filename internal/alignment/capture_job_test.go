package alignment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCaptureEndpointMustBeThePreparedEndpoint(t *testing.T) {
	b := boundaryFixture()
	p := Plan{ID: "plan", Source: Observation{NodeID: b.OriginNodeID}, Target: Observation{NodeID: "other"}}
	p.Source.Schema.Database, p.Source.Schema.Table = b.DatabaseName, b.TableName
	probe := &SnapshotCapture{planID: p.ID, base: b}
	if err := probe.requireEndpoint(p, b.OriginNodeID); err != nil {
		t.Fatal(err)
	}
	for _, variant := range []*SnapshotCapture{nil, {}, {planID: "wrong", base: b}, {planID: p.ID, base: b, closed: true}} {
		if err := variant.requireEndpoint(p, b.OriginNodeID); err == nil {
			t.Fatal("invalid probe accepted")
		}
	}
	p.Source.Schema.Table = "changed"
	if err := probe.requireEndpoint(p, b.OriginNodeID); err == nil {
		t.Fatal("changed scope accepted")
	}
}

func TestCaptureMarkerPersistenceIsSingleAssignment(t *testing.T) {
	for _, rows := range []int64{0, 1, 2} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectBegin()
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectExec("UPDATE sync_alignment_job SET capture_marker=.*capture_marker IS NULL AND capture_boundary IS NULL").WithArgs("token", "job", JobPrepared).WillReturnResult(sqlmock.NewResult(0, rows))
		err = persistCaptureMarker(context.Background(), tx, "job", "token")
		if (err == nil) != (rows == 1) {
			t.Fatal("incorrect transition count", rows, err)
		}
		mock.ExpectRollback()
		_ = tx.Rollback()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		_ = db.Close()
	}
}

func TestCaptureBoundaryPersistenceChecksServerAndMarker(t *testing.T) {
	for _, match := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		b := boundaryFixture()
		uuid := "different"
		if match {
			uuid = b.MySQLServerUUID
		}
		mock.ExpectQuery("SELECT @@server_uuid").WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow(uuid))
		if match {
			mock.ExpectExec("UPDATE sync_alignment_job SET capture_boundary=.*capture_marker=.*capture_boundary IS NULL").WithArgs(sqlmock.AnyArg(), "job", JobTargetCommitted, b.OriginNodeID, b.Token).WillReturnResult(sqlmock.NewResult(0, 1))
		}
		err = persistCaptureBoundary(context.Background(), db, "job", JobTargetCommitted, b)
		if (err == nil) != match {
			t.Fatal("wrong server accepted", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		_ = db.Close()
	}
}

func TestJobUpdatePropagatesDriverFailure(t *testing.T) {
	want := errors.New("database failed")
	if !errors.Is(requireJobUpdate(nil, want), want) {
		t.Fatal("driver failure hidden")
	}
	if err := requireJobUpdate(sqlmock.NewErrorResult(sql.ErrConnDone), nil); !errors.Is(err, sql.ErrConnDone) {
		t.Fatal(err)
	}
}

type disconnectedProbeClient struct{ probeClient }

func (*disconnectedProbeClient) Connect(context.Context) error {
	return errors.New("owned_connect_attempted")
}

func TestExpiredPlanAllowsOnlyExistingCommitCaptureRecovery(t *testing.T) {
	rule, left, right := fixture()
	left.HasRows = true
	plan, err := BuildPlan(rule, left, right, time.Now().Add(-10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := StartSnapshotCapture(context.Background(), nil, nil, "example", plan, rule, right.NodeID, true); err == nil || err.Error() != "alignment_plan_expired" {
		t.Fatal("expired copy allowed", err)
	}
	for _, phase := range []string{JobPrepared, JobTargetCommitted} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		id, scope, _, err := jobIdentity(plan, right.NodeID)
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(plan)
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectQuery("SELECT @@lower_case_table_names").WillReturnRows(sqlmock.NewRows([]string{"folding"}).AddRow(0))
		rows := sqlmock.NewRows([]string{"scope", "plan_id", "node", "role", "phase", "plan", "count", "digest", "marker", "boundary"})
		if phase == JobTargetCommitted {
			rows.AddRow(scope, plan.ID, right.NodeID, "TARGET", phase, b, 2, strings.Repeat("b", 64), strings.Repeat("a", 64), nil)
			mock.ExpectQuery("SELECT scope_hash,plan_id.*WHERE job_id").WithArgs(id).WillReturnRows(rows)
			mock.ExpectQuery(`SELECT DATABASE\(\),@@server_uuid`).WillReturnRows(sqlmock.NewRows([]string{"db", "uuid"}).AddRow("control", boundaryFixture().MySQLServerUUID))
		} else {
			rows.AddRow(scope, plan.ID, right.NodeID, "TARGET", phase, b, nil, nil, nil, nil)
			mock.ExpectQuery("SELECT scope_hash,plan_id.*WHERE job_id").WithArgs(id).WillReturnRows(rows)
		}
		_, err = StartSnapshotRecoveryCapture(context.Background(), db, &disconnectedProbeClient{}, "example", plan, rule, right.NodeID, true)
		want := "alignment_recovery_capture_not_committed"
		if phase == JobTargetCommitted {
			want = "owned_connect_attempted"
		}
		if err == nil || err.Error() != want {
			t.Fatal("expired recovery authorization", phase, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		_ = db.Close()
	}
}
