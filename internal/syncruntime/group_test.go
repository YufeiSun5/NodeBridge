package syncruntime

import (
	"context"
	"strings"
	"testing"
)

func TestWorkerGroupRunsWorkersAndReturnsFirstError(t *testing.T) {
	group := WorkerGroup{Workers: []Worker{
		{},
	}}

	err := group.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "stepper is required") {
		t.Fatalf("expected first error, got %v", err)
	}
}

func TestWorkerGroupAllowsEmptyGroup(t *testing.T) {
	if err := (WorkerGroup{}).Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}
