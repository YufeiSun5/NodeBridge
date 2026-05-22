package apply

import (
	"context"

	"github.com/YufeiSun5/NodeBridge/internal/mapper"
)

type Result struct {
	EventID        string
	SourceTable    string
	TargetTable    string
	AlreadyApplied bool
}

type Worker interface {
	Apply(ctx context.Context, event mapper.MappedEvent) (Result, error)
}

type BatchResult struct {
	Results []Result
}

type BatchWorker interface {
	ApplyBatch(ctx context.Context, events []mapper.MappedEvent) (BatchResult, error)
}
