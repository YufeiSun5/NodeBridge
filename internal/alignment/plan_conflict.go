package alignment

import (
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

// Existing receipts remain authoritative for their original mapping. Changing a
// rule cannot turn an old receipt into authorization for a different database.
func checkExistingPairPlans(rule rules.SyncRule, local, peer *SnapshotJob) error {
	local, peer = pendingPairJobs(local, peer)
	rule = canonicalCutoverRule(rule)
	for _, job := range []*SnapshotJob{local, peer} {
		if job == nil || job.Plan.RuleHash == hash(rule) {
			continue
		}
		left, right := job.Plan.Source, job.Plan.Target
		if job.Plan.Direction == ToLeft {
			left, right = right, left
		}
		targetDB, targetTable := rule.TargetDatabaseName, rule.TargetTableName
		if targetDB == "" {
			targetDB = rule.DatabaseName
		}
		if targetTable == "" {
			targetTable = rule.TableName
		}
		return fmt.Errorf("alignment_existing_plan_rule_changed: rule_id=%q node_id=%q job_id=%q plan_id=%q phase=%q old_mapping=%q.%q->%q.%q requested_mapping=%q.%q->%q.%q; explicit migration is required; retrying or deleting rule files does not retire persisted history",
			rule.ID, job.NodeID, job.ID, job.Plan.ID, job.Phase,
			left.Schema.Database, left.Schema.Table, right.Schema.Database, right.Schema.Table,
			rule.DatabaseName, rule.TableName, targetDB, targetTable)
	}
	return nil
}
