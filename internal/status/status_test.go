package status_test

import (
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/status"
)

func TestOverviewFields(t *testing.T) {
	overview := status.Overview{
		ProductName:        "DataSync",
		Mode:               "edge",
		AgentStatus:        status.AgentRunning,
		UploadQueueDepth:   12,
		DownlinkQueueDepth: 3,
		FailedEventCount:   1,
		ConflictCount:      2,
		Version:            "0.1.0",
	}

	if overview.ProductName != "DataSync" {
		t.Fatalf("unexpected product name %q", overview.ProductName)
	}
	if overview.AgentStatus != status.AgentRunning {
		t.Fatalf("unexpected agent status %q", overview.AgentStatus)
	}
	if overview.UploadQueueDepth != 12 || overview.DownlinkQueueDepth != 3 {
		t.Fatalf("unexpected queue depths %+v", overview)
	}
}
