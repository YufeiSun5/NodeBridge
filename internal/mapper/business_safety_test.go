package mapper_test

import (
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestRejectsIncompleteCompositeKey(t *testing.T) {
	for _, key := range []map[string]any{nil, {"id": 7}, {"id": 7, "tenant_id": nil}, {"id": 7, "tenant_id": 2, "extra": 9}} {
		evt := baseEvent()
		evt.PrimaryKey = key
		_, err := mapper.MapEvent(evt, rules.SyncRule{PrimaryKeys: []string{"id", "tenant_id"}})
		if err == nil || !strings.Contains(err.Error(), "invalid_primary_key") {
			t.Fatalf("key=%v: %v", key, err)
		}
	}
}

func TestRejectsPrimaryKeyChange(t *testing.T) {
	evt := baseEvent()
	evt.EventType = event.TypeUpdate
	evt.Before["id"], evt.After["id"] = int64(1), int64(2)
	evt.PrimaryKey["id"] = int64(2)
	_, err := mapper.MapEvent(evt, rules.SyncRule{PrimaryKeys: []string{"id"}})
	if err == nil || !strings.Contains(err.Error(), "primary_key_changed") {
		t.Fatalf("result: %v", err)
	}
}

func TestRejectsExplicitAndIdentityColumnCollision(t *testing.T) {
	evt := baseEvent()
	evt.After["source_name"] = "other"
	for range 50 {
		_, err := mapper.MapEvent(evt, rules.SyncRule{PrimaryKeys: []string{"id"}, ColumnMappings: []rules.ColumnMapping{{SourceColumn: "source_name", TargetColumn: "name"}}})
		if err == nil || !strings.Contains(err.Error(), "column_mapping_collision") {
			t.Fatalf("result: %v", err)
		}
	}
}

func TestExplicitTargetKeyAlsoMapsRowImage(t *testing.T) {
	rule := rules.SyncRule{PrimaryKeys: []string{"id"}, TargetPrimaryKeys: []string{"target_id"}}
	mapped, err := mapper.MapEvent(baseEvent(), rule)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := mapped.TargetAfter["id"]; exists || mapped.TargetAfter["target_id"] != mapped.TargetPrimaryKey["target_id"] {
		t.Fatalf("inconsistent target key: %+v", mapped)
	}
	rule.ColumnMappings = []rules.ColumnMapping{{SourceColumn: "id", TargetColumn: "other_id"}}
	if _, err := mapper.MapEvent(baseEvent(), rule); err == nil || !strings.Contains(err.Error(), "primary_key_mapping_mismatch") {
		t.Fatalf("conflicting explicit key mapping accepted: %v", err)
	}
}
