package alignment

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCancelUncommittedJob(t *testing.T) {
	for _, tc := range []struct {
		name, nodeRole, phase, peerPhase string
		cutover, writeFailure, wantOK    bool
	}{
		{name: "target_first", nodeRole: "TARGET", phase: JobPrepared, peerPhase: JobPrepared, wantOK: true},
		{name: "source_after_target", nodeRole: "SOURCE", phase: JobPrepared, peerPhase: JobCancelled, wantOK: true},
		{name: "source_before_target", nodeRole: "SOURCE", phase: JobPrepared, peerPhase: JobPrepared},
		{name: "cutover_exists", nodeRole: "TARGET", phase: JobPrepared, peerPhase: JobPrepared, cutover: true},
		{name: "transaction_rolls_back", nodeRole: "TARGET", phase: JobPrepared, peerPhase: JobPrepared, writeFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule, left, right := fixture()
			plan, err := BuildPlan(rule, left, right, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			node, peerNode := right.NodeID, left.NodeID
			if tc.nodeRole == "SOURCE" {
				node, peerNode = peerNode, node
			}
			id, scope, role, _ := jobIdentity(plan, node)
			peerID, _, peerRole, _ := jobIdentity(plan, peerNode)
			peer := SnapshotJob{ID: peerID, NodeID: peerNode, Role: peerRole, Plan: plan, Phase: tc.peerPhase}
			encoded, _ := json.Marshal(plan)
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT @@lower_case_table_names").WillReturnRows(sqlmock.NewRows([]string{"folding"}).AddRow(0))
			mock.ExpectQuery("SELECT scope_hash,plan_id.*FOR UPDATE").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"scope", "plan", "node", "role", "phase", "json", "count", "digest", "marker", "boundary"}).AddRow(scope, plan.ID, node, role, tc.phase, encoded, nil, nil, nil, nil))
			if tc.name != "source_before_target" {
				count := 0
				if tc.cutover {
					count = 1
				}
				mock.ExpectQuery("SELECT COUNT.*sync_alignment_cutover").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
				if !tc.cutover {
					update := mock.ExpectExec("UPDATE sync_alignment_job SET phase").WithArgs(JobCancelled, hash([]string{"cancelled", id}), id, tc.phase)
					if tc.writeFailure {
						update.WillReturnError(errors.New("write failed"))
					} else {
						update.WillReturnResult(sqlmock.NewResult(0, 1))
					}
				}
			}
			if tc.wantOK {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			job, err := CancelUncommittedJob(context.Background(), db, plan, node, peer)
			if (err == nil) != tc.wantOK || (tc.wantOK && job.Phase != JobCancelled) {
				t.Fatal(job, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPendingPairJobs(t *testing.T) {
	for _, tc := range []struct {
		local, peer         string
		wantLocal, wantPeer bool
	}{
		{JobCancelled, JobCancelled, false, false},
		{JobCancelled, JobPrepared, true, true},
		{JobSourceReady, JobCancelled, true, true},
		{JobCancelled, "", false, false},
		{"", JobCancelled, false, false},
		{JobPrepared, "", true, false},
	} {
		var local, peer *SnapshotJob
		if tc.local != "" {
			local = &SnapshotJob{Phase: tc.local, Plan: Plan{ID: "same"}}
		}
		if tc.peer != "" {
			peer = &SnapshotJob{Phase: tc.peer, Plan: Plan{ID: "same"}}
		}
		local, peer = pendingPairJobs(local, peer)
		if (local != nil) != tc.wantLocal || (peer != nil) != tc.wantPeer {
			t.Fatal(tc, local, peer)
		}
	}
}
