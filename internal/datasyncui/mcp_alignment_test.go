package datasyncui

import (
	"context"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
)

func TestMCPInitialAlignmentConfirmationStatusInterruptAndRetry(t *testing.T) {
	_, config, rulePath := newTempApp(t)
	service, err := NewMCPService(config, rulePath, "", true)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	service.app.alignment.run = func(ctx context.Context, _, _ string, req alignment.ConfiguredRequest, progress func(string)) (alignment.CutoverProof, error) {
		if req.PeerID != "edge-1,edge-2" || req.RuleID != "pair" || !req.Confirm {
			t.Error("MCP lost explicit topology request")
		}
		progress("copying")
		close(entered)
		<-ctx.Done()
		return alignment.CutoverProof{}, ctx.Err()
	}
	args := `{"rule_id":"pair","peer_node_id":"edge-1,edge-2","confirm":true}`
	if response := callMCP(t, service, "nodebridge_start_initial_alignment", strings.Replace(args, "true", "false", 1)); !strings.Contains(response, `"isError":true`) {
		t.Fatal("unconfirmed MCP copy accepted", response)
	}
	if response := callMCP(t, service, "nodebridge_start_initial_alignment", args); strings.Contains(response, `"isError":true`) {
		t.Fatal(response)
	}
	<-entered
	if response := callMCP(t, service, "nodebridge_initial_alignment_status", `{}`); !strings.Contains(response, "copying") {
		t.Fatal(response)
	}
	if response := callMCP(t, service, "nodebridge_start_initial_alignment", args); !strings.Contains(response, "alignment_operation_running") {
		t.Fatal("duplicate start accepted", response)
	}
	if response := callMCP(t, service, "nodebridge_interrupt_initial_alignment", `{}`); strings.Contains(response, `"isError":true`) {
		t.Fatal(response)
	}
	if state := waitAlignmentStatus(t, service.app); state.Stage != "failed" || state.Running {
		t.Fatal(state)
	}
	service.app.alignment.run = func(context.Context, string, string, alignment.ConfiguredRequest, func(string)) (alignment.CutoverProof, error) {
		return alignment.CutoverProof{Plan: alignment.Plan{ID: "reconciled"}, Result: alignment.CopyResult{Rows: 1}}, nil
	}
	if response := callMCP(t, service, "nodebridge_start_initial_alignment", args); strings.Contains(response, `"isError":true`) {
		t.Fatal(response)
	}
	if state := waitAlignmentStatus(t, service.app); state.Stage != "completed" || state.PlanID != "reconciled" {
		t.Fatal(state)
	}
	if response := callMCP(t, service, "nodebridge_initial_alignment_status", `{}`); !strings.Contains(response, "completed") {
		t.Fatal(response)
	}
}
