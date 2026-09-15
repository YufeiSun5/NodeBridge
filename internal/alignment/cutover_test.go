package alignment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/normalizer"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
)

func cutoverFixture(t *testing.T) (CutoverProof, SnapshotJob, SnapshotJob) {
	t.Helper()
	rule, left, right := fixture()
	left.HasRows = true
	plan, err := BuildPlan(rule, left, right, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	result := &CopyResult{PlanID: plan.ID, Rows: 2, Digest: strings.Repeat("a", 64)}
	jobs := make([]SnapshotJob, 0, 2)
	for i, observed := range []Observation{left, right} {
		boundary := boundaryFixture()
		boundary.OriginNodeID, boundary.DatabaseName, boundary.TableName = observed.NodeID, observed.Schema.Database, observed.Schema.Table
		boundary.BinlogPos = uint32(1000 + i*100)
		id, _, role, _ := jobIdentity(plan, observed.NodeID)
		phase := JobTargetConfirmed
		if role == "TARGET" {
			phase = JobTargetCommitted
		}
		jobs = append(jobs, SnapshotJob{ID: id, NodeID: observed.NodeID, Role: role, Phase: phase, Plan: plan, Result: result, CaptureMarker: boundary.Token, CaptureBoundary: &boundary})
	}
	proof, err := BuildCutoverProof(rule, jobs[0], jobs[1])
	if err != nil {
		t.Fatal(err)
	}
	return proof, jobs[0], jobs[1]
}

func TestCutoverRequiresMatchingCopyAndBothBoundaries(t *testing.T) {
	proof, source, target := cutoverFixture(t)
	if proof.Validate() != nil {
		t.Fatal("valid proof rejected")
	}
	for _, mode := range []string{"source_pending", "target_pending", "no_boundary", "receipt_changed", "marker_changed", "job_changed", "plan_changed"} {
		t.Run(mode, func(t *testing.T) {
			s, d := source, target
			switch mode {
			case "source_pending":
				s.Phase = JobSourceReady
			case "target_pending":
				d.Phase = JobPrepared
			case "no_boundary":
				d.CaptureBoundary = nil
			case "receipt_changed":
				d.Result = &CopyResult{PlanID: proof.Plan.ID, Rows: 9, Digest: proof.Result.Digest}
			case "marker_changed":
				d.CaptureMarker = strings.Repeat("b", 64)
			case "job_changed":
				d.ID = strings.Repeat("b", 64)
			case "plan_changed":
				d.Plan.ID = strings.Repeat("b", 64)
			}
			if _, err := BuildCutoverProof(proof.Rule, s, d); err == nil {
				t.Fatal("incomplete copy accepted")
			}
		})
	}
	proof.Target.TableName = "different"
	proof.ID = ""
	proof.ID = hash(proof)
	if err := proof.Validate(); err == nil {
		t.Fatal("resealed foreign scope accepted")
	}
}

func TestCutoverFilterPreservesScopeAndSourceLineage(t *testing.T) {
	proof, _, _ := cutoverFixture(t)
	filter, err := NewCutoverFilter("server", nil, []CutoverProof{proof})
	if err != nil {
		t.Fatal(err)
	}
	base := event.SyncEvent{EventID: "legacy", OriginNodeID: "edge", SourceNodeID: "edge", DatabaseName: proof.Source.DatabaseName, TableName: proof.Source.TableName, BinlogFile: proof.Source.BinlogFile, BinlogPos: proof.Source.BinlogPos + 10}
	if _, superseded, err := filter.Superseded(base); err != nil || !superseded {
		t.Fatal("legacy not classified", err)
	}
	for _, mode := range []string{"current", "before", "other_table", "other_origin", "wrong_epoch", "wrong_uuid", "wrong_target", "wrong_sender", "no_position"} {
		t.Run(mode, func(t *testing.T) {
			evt := base
			evt.Headers = map[string]string{EpochHeader: proof.ID, SourceUUIDHeader: proof.Source.MySQLServerUUID}
			wantError, wantSuperseded := false, false
			switch mode {
			case "before":
				evt.BinlogPos--
				evt.BinlogPos -= 10
				wantSuperseded = true
			case "other_table":
				evt.TableName = "unrelated"
				evt.Headers = nil
			case "other_origin":
				evt.OriginNodeID = "unrelated"
				evt.Headers = nil
			case "wrong_epoch":
				evt.Headers[EpochHeader] = strings.Repeat("b", 64)
				wantError = true
			case "wrong_uuid":
				evt.Headers[SourceUUIDHeader] = "changed"
				wantError = true
			case "wrong_target":
				evt.TargetNodeID = "third"
				wantError = true
			case "wrong_sender":
				evt.SourceNodeID = "third"
				wantError = true
			case "no_position":
				evt.BinlogPos = 0
				wantError = true
			}
			_, superseded, err := filter.Superseded(evt)
			if (err != nil) != wantError || superseded != wantSuperseded {
				t.Fatalf("classification %v %v", superseded, err)
			}
		})
	}
	if _, err := NewCutoverFilter("server", nil, []CutoverProof{proof, proof}); err == nil {
		t.Fatal("overlapping proofs accepted")
	}
}

func TestCutoverCaptureAndNormalizer(t *testing.T) {
	proof, _, _ := cutoverFixture(t)
	filter, err := NewCutoverFilter("edge", nil, []CutoverProof{proof})
	if err != nil {
		t.Fatal(err)
	}
	change := cdc.ChangeEvent{DatabaseName: proof.Source.DatabaseName, TableName: proof.Source.TableName, Operation: cdc.OperationInsert, PrimaryKey: map[string]any{"id": "1"}, After: map[string]any{"id": "1"}, BinlogFile: proof.Source.BinlogFile, BinlogPos: proof.Source.BinlogPos - 1, EventTime: time.Now()}
	if skip, err := filter.SkipChange(change); err != nil || !skip {
		t.Fatal(skip, err)
	}
	n := CutoverNormalizer{Normalizer: normalizer.New(normalizer.Options{NodeID: "edge"}), Filter: filter}
	if _, err := n.Normalize(change); err == nil {
		t.Fatal("old change stamped as current")
	}
	change.BinlogPos += 2
	a, err := n.Normalize(change)
	if err != nil {
		t.Fatal(err)
	}
	b, err := n.Normalize(change)
	if err != nil || a.EventID != b.EventID || a.Headers[EpochHeader] != proof.ID || a.Headers[SourceUUIDHeader] != proof.Source.MySQLServerUUID {
		t.Fatal(a, b, err)
	}
	plain, err := n.Normalizer.Normalize(change)
	if err != nil || plain.EventID == a.EventID {
		t.Fatal("new epoch reused old event identity")
	}
	change.TableName = "unrelated"
	if skip, err := filter.SkipChange(change); err != nil || skip {
		t.Fatal("unrelated capture suppressed", err)
	}
}

type cutoverMessage struct {
	body          []byte
	acked, nacked bool
	ackError      error
}

func (m *cutoverMessage) Body() []byte               { return m.body }
func (m *cutoverMessage) Ack(bool) error             { m.acked = m.ackError == nil; return m.ackError }
func (m *cutoverMessage) Nack(_, requeue bool) error { m.nacked = requeue; return nil }

type cutoverBatch []rabbitmq.IncomingMessage

func (b cutoverBatch) GetBatch(context.Context, int, time.Duration) ([]rabbitmq.IncomingMessage, error) {
	return b, nil
}

func TestCutoverMessageAuditBeforeAckAndFailureRequeues(t *testing.T) {
	for _, mode := range []string{"success", "audit_failed", "ack_failed"} {
		t.Run(mode, func(t *testing.T) {
			proof, _, _ := cutoverFixture(t)
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			filter, err := NewCutoverFilter("server", db, []CutoverProof{proof})
			if err != nil {
				t.Fatal(err)
			}
			body, _ := json.Marshal(event.SyncEvent{EventID: "old", SourceNodeID: "edge", OriginNodeID: "edge", DatabaseName: proof.Source.DatabaseName, TableName: proof.Source.TableName})
			old := &cutoverMessage{body: body}
			other := &cutoverMessage{body: []byte(`{"event_id":"other","origin_node_id":"other"}`)}
			q := mock.ExpectExec("INSERT INTO sync_alignment_event").WithArgs(sqlmock.AnyArg(), proof.ID, "old", body, "snapshot_superseded")
			if mode == "audit_failed" {
				q.WillReturnError(errors.New("audit failed"))
			} else {
				q.WillReturnResult(sqlmock.NewResult(1, 1))
			}
			if mode == "ack_failed" {
				old.ackError = errors.New("ack failed")
			}
			kept, err := (CutoverMessages{Source: cutoverBatch{other, old}, Filter: filter}).GetBatch(context.Background(), 10, time.Millisecond)
			if mode == "success" {
				if err != nil || !old.acked || old.nacked || other.acked || other.nacked || len(kept) != 1 || kept[0] != other {
					t.Fatalf("wrong delivery handling: %v", err)
				}
			} else if err == nil || old.acked || !old.nacked || !other.nacked || len(kept) != 0 {
				t.Fatalf("unsafe failed batch: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
