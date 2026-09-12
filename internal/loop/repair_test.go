package loop_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type repairLog struct {
	fakeApplyLog
	operation string
	fail      bool
}

func (l repairLog) IsRepairReplay(_ context.Context, id, node, database, table, operation string) (bool, error) {
	if l.fail {
		return false, errors.New("proof unavailable")
	}
	return id == "repair-id" && node == "edge-001" && database == "scada_edge" && table == "device_config" && (operation == l.operation || operation == "UPDATE" && l.operation == "DELETE"), nil
}

func TestOwnRepairProofDoesNotSuppressSubsequentBusinessWrite(t *testing.T) {
	for _, operation := range []cdc.Operation{cdc.OperationInsert, cdc.OperationUpdate, cdc.OperationDelete} {
		for _, proof := range []string{"INSERT", "UPDATE", "DELETE", "none"} {
			set := ruleSet(rules.SyncRule{Enable: true, Direction: rules.DirectionBidirectional})
			s := loop.NewSuppressor("edge-001", set, repairLog{operation: proof})
			change := cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config", Operation: operation, Before: map[string]any{"last_event_id": "old", "updated_by_node": "edge-001"}, After: map[string]any{"last_event_id": "repair-id", "updated_by_node": "edge-001"}}
			if operation == cdc.OperationDelete {
				change.Before = change.After
				change.After = nil
			}
			decision := mustDecision(t, s, change)
			wantSuppress := string(operation) == proof || operation == cdc.OperationUpdate && proof == "DELETE"
			if decision.Upload == wantSuppress {
				t.Fatalf("operation=%s proof=%s decision=%+v", operation, proof, decision)
			}
			if operation == cdc.OperationUpdate {
				change.Before["last_event_id"] = "repair-id"
				if !mustDecision(t, s, change).Upload {
					t.Fatal("later local write retaining repair markers was suppressed")
				}
			}
		}
	}
}

func TestOwnMarkerChangeRequiresRepairProofNotAnOldApplyReceipt(t *testing.T) {
	set := ruleSet(rules.SyncRule{Enable: true, Direction: rules.DirectionBidirectional})
	change := cdc.ChangeEvent{DatabaseName: "scada_edge", TableName: "device_config", Operation: cdc.OperationUpdate, Before: map[string]any{"last_event_id": "old-remote", "updated_by_node": "remote"}, After: map[string]any{"last_event_id": "old-remote", "updated_by_node": "edge-001"}}
	s := loop.NewSuppressor("edge-001", set, repairLog{fakeApplyLog: fakeApplyLog{"old-remote": true}, operation: "UPDATE"})
	if !mustDecision(t, s, change).Upload {
		t.Fatal("local ownership stamp was mistaken for repair")
	}
	change.After["last_event_id"] = "repair-id"
	s = loop.NewSuppressor("edge-001", set, repairLog{fail: true})
	if _, err := s.ShouldUpload(context.Background(), change); err == nil {
		t.Fatal("proof failure was ignored")
	}
}
