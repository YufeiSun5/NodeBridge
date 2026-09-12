package syncruntime

import (
	"context"
	"errors"
)

type ConflictRepairer interface {
	RunOnce(context.Context) (bool, error)
}

// ConflictRepairRuntime uses the standard worker retry and status lifecycle.
type ConflictRepairRuntime struct {
	Repairer ConflictRepairer
}

func (r ConflictRepairRuntime) RunOnce(ctx context.Context) (StepResult, error) {
	result := StepResult{Action: "conflict_repair"}
	if r.Repairer == nil {
		return result, errors.New("conflict_repairer_required")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	repaired, err := r.Repairer.RunOnce(ctx)
	if err != nil {
		return result, err
	}
	result.Processed = repaired
	if repaired {
		result.Count = 1
	}
	return result, nil
}
