package main

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/capture"
	"github.com/YufeiSun5/NodeBridge/internal/conflict"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncruntime"
)

func needsConflictRuntime(set *rules.RuleSet) bool {
	if set != nil {
		for _, rule := range set.Rules {
			if rule.Enable && rule.Direction == rules.DirectionBidirectional && rule.ConflictPolicy == rules.ConflictLastWriteWin {
				return true
			}
		}
	}
	return false
}

func conflictCaptureFilter(filter, database string) string {
	if strings.TrimSpace(filter) == "" {
		return filter
	}
	return "(" + filter + ")|(" + regexp.QuoteMeta(database) + `\.` + capture.Table + ")"
}

// Called before starting any worker. Public rule validation remains the feature gate.
func attachConflictRuntime(cfg *appconfig.Config, set *rules.RuleSet, db *sql.DB, worker *apply.SQLWorker, source syncruntime.CanalBatchSource) (syncruntime.CanalBatchSource, syncruntime.LocalVersionRecorder, *syncruntime.ConflictRepairRuntime, error) {
	if !needsConflictRuntime(set) {
		return source, nil, nil, nil
	}
	if cfg == nil || !strings.EqualFold(cfg.CDC.Type, "canal") || worker == nil || worker.DB != db || worker.CaptureFence != nil {
		return nil, nil, nil, errors.New("conflict_runtime_dependencies_invalid")
	}
	fence, err := capture.NewFence(db, cfg.MySQL.Database, cfg.Node.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	wrapped, err := capture.NewSource(source, fence)
	if err != nil {
		return nil, nil, nil, err
	}
	worker.CaptureFence = fence
	recorder := conflict.LocalRecorder{DB: db, NodeID: cfg.Node.ID, Rules: set}
	repair := &syncruntime.ConflictRepairRuntime{Repairer: apply.RepairWorker{DB: db, NodeID: cfg.Node.ID, Rules: set, CaptureFence: fence}}
	return wrapped, recorder, repair, nil
}
