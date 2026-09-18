package alignment

import (
	"strings"
	"testing"
	"time"
)

func TestExistingPairPlanConflictIdentifiesLocalOrPeerOldDatabase(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		rule, left, right := fixture()
		left.HasRows, right.HasRows = !reverse, reverse
		rule = canonicalCutoverRule(rule)
		plan, err := BuildPlan(rule, left, right, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		job := &SnapshotJob{ID: "old-job", NodeID: "edge", Plan: plan, Phase: JobTargetConfirmed}
		if err := checkExistingPairPlans(rule, job, nil); err != nil {
			t.Fatal(err)
		}
		rule.Name = "中文名称"
		if err := checkExistingPairPlans(rule, job, nil); err != nil {
			t.Fatal("display name must not require migration", err)
		}
		rule.TargetDatabaseName = "new_main"
		for _, jobs := range [][2]*SnapshotJob{{job, nil}, {nil, job}, {job, job}} {
			err := checkExistingPairPlans(rule, jobs[0], jobs[1])
			if err == nil {
				t.Fatal("old mapping accepted")
			}
			for _, want := range []string{"alignment_existing_plan_rule_changed", "old-job", plan.ID, `old_mapping="left_db"."source_rows"->"right_db"."target_rows"`, `requested_mapping="left_db"."source_rows"->"new_main"."target_rows"`} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("missing %q in %v", want, err)
				}
			}
		}
		if hash(job.Plan) != hash(plan) || job.Phase != JobTargetConfirmed {
			t.Fatal("history was changed")
		}
	}
}

func TestCancelledPlanConflictStillRequiresPeerCancellation(t *testing.T) {
	rule, left, right := fixture()
	rule = canonicalCutoverRule(rule)
	plan, err := BuildPlan(rule, left, right, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	local := &SnapshotJob{ID: "local", Phase: JobCancelled, Plan: plan}
	peer := &SnapshotJob{ID: "peer", Phase: JobPrepared, Plan: plan}
	rule.TargetDatabaseName = "new_main"
	if checkExistingPairPlans(rule, local, peer) == nil {
		t.Fatal("unresolved peer attempt ignored")
	}
	peer.Phase = JobCancelled
	if err := checkExistingPairPlans(rule, local, peer); err != nil {
		t.Fatal("fully cancelled attempt blocked replanning", err)
	}
	if err := checkExistingPairPlans(rule, nil, nil); err != nil {
		t.Fatal(err)
	}
}
