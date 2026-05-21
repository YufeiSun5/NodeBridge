package mapper_test

import (
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestMapEventDefaultsToSourceNames(t *testing.T) {
	mapped, err := mapper.MapEvent(baseEvent(), rules.SyncRule{PrimaryKeys: []string{"id"}})
	if err != nil {
		t.Fatalf("MapEvent returned error: %v", err)
	}

	if mapped.TargetDatabase != "scada_edge" || mapped.TargetTable != "device_config" {
		t.Fatalf("unexpected target %s.%s", mapped.TargetDatabase, mapped.TargetTable)
	}
	if mapped.TargetAfter["name"] != "pump-a" {
		t.Fatalf("expected same column name, got %+v", mapped.TargetAfter)
	}
}

func TestMapEventMapsTargetTable(t *testing.T) {
	mapped, err := mapper.MapEvent(baseEvent(), rules.SyncRule{
		TargetDatabaseName: "scada_center",
		TargetTableName:    "device_settings",
		PrimaryKeys:        []string{"id"},
	})
	if err != nil {
		t.Fatalf("MapEvent returned error: %v", err)
	}

	if mapped.Event.DatabaseName != "scada_center" || mapped.Event.TableName != "device_settings" {
		t.Fatalf("unexpected mapped event target %s.%s", mapped.Event.DatabaseName, mapped.Event.TableName)
	}
}

func TestMapEventMapsColumnsAndPrimaryKey(t *testing.T) {
	mapped, err := mapper.MapEvent(baseEvent(), rules.SyncRule{
		TargetTableName: "device_settings",
		PrimaryKeys:     []string{"id"},
		ColumnMappings: []rules.ColumnMapping{
			{SourceColumn: "id", TargetColumn: "setting_id"},
			{SourceColumn: "name", TargetColumn: "display_name"},
			{SourceColumn: "value", TargetColumn: "setting_value"},
		},
	})
	if err != nil {
		t.Fatalf("MapEvent returned error: %v", err)
	}

	if mapped.TargetPrimaryKey["setting_id"] != int64(7) {
		t.Fatalf("unexpected primary key %+v", mapped.TargetPrimaryKey)
	}
	if mapped.TargetAfter["display_name"] != "pump-a" {
		t.Fatalf("expected mapped after column, got %+v", mapped.TargetAfter)
	}
	if mapped.TargetBefore["setting_value"] != "old" {
		t.Fatalf("expected mapped before column, got %+v", mapped.TargetBefore)
	}
}

func TestMapEventFiltersBeforeMapping(t *testing.T) {
	mapped, err := mapper.MapEvent(baseEvent(), rules.SyncRule{
		PrimaryKeys:    []string{"id"},
		IncludeColumns: []string{"id", "name", "ignored"},
		ExcludeColumns: []string{"ignored"},
		ColumnMappings: []rules.ColumnMapping{{SourceColumn: "name", TargetColumn: "display_name"}},
	})
	if err != nil {
		t.Fatalf("MapEvent returned error: %v", err)
	}

	if _, ok := mapped.TargetAfter["value"]; ok {
		t.Fatalf("value should be filtered before mapping: %+v", mapped.TargetAfter)
	}
	if _, ok := mapped.TargetAfter["ignored"]; ok {
		t.Fatalf("ignored should be excluded: %+v", mapped.TargetAfter)
	}
	if mapped.TargetAfter["display_name"] != "pump-a" {
		t.Fatalf("expected display_name, got %+v", mapped.TargetAfter)
	}
}

func TestMapEventRejectsPrimaryKeyCountMismatch(t *testing.T) {
	_, err := mapper.MapEvent(baseEvent(), rules.SyncRule{
		PrimaryKeys:       []string{"id", "tenant_id"},
		TargetPrimaryKeys: []string{"setting_id"},
	})
	if err == nil {
		t.Fatal("expected primary key count mismatch")
	}
}

func TestMapEventRejectsInvalidTargetTable(t *testing.T) {
	_, err := mapper.MapEvent(baseEvent(), rules.SyncRule{
		TargetTableName: "device_settings;drop",
		PrimaryKeys:     []string{"id"},
	})
	if err == nil {
		t.Fatal("expected invalid identifier error")
	}
}

func TestMapEventRejectsInvalidTargetColumn(t *testing.T) {
	_, err := mapper.MapEvent(baseEvent(), rules.SyncRule{
		PrimaryKeys:    []string{"id"},
		ColumnMappings: []rules.ColumnMapping{{SourceColumn: "name", TargetColumn: "display-name"}},
	})
	if err == nil {
		t.Fatal("expected invalid identifier error")
	}
}

func baseEvent() event.SyncEvent {
	return event.SyncEvent{
		EventID:      "evt-001",
		EventType:    event.TypeUpdate,
		OriginNodeID: "edge-001",
		SourceNodeID: "edge-001",
		DatabaseName: "scada_edge",
		TableName:    "device_config",
		PrimaryKey:   map[string]any{"id": int64(7)},
		Before: map[string]any{
			"id":      int64(7),
			"name":    "pump-a-old",
			"value":   "old",
			"ignored": "before",
		},
		After: map[string]any{
			"id":      int64(7),
			"name":    "pump-a",
			"value":   "new",
			"ignored": "after",
		},
	}
}
