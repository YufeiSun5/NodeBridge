package conflict_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/conflict"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/normalizer"
	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
)

func TestVersionSurvivesRetryRelayAndJSON(t *testing.T) {
	change := cdc.ChangeEvent{DatabaseName: "source", TableName: "items", Operation: cdc.OperationUpdate, PrimaryKey: map[string]any{"id": json.Number("9007199254740993")}, Before: map[string]any{"id": json.Number("9007199254740993"), "bytes": rowvalue.Binary{0, 255}}, After: map[string]any{"id": json.Number("9007199254740993"), "bytes": rowvalue.Binary{128, 255}}, EventTime: time.Unix(1000, 123000000), BinlogFile: "mysql-bin.000001", BinlogPos: 123}
	n := normalizer.New(normalizer.Options{NodeID: "edge", Now: func() time.Time { return time.Unix(2000, 0) }})
	first, err := n.Normalize(change)
	if err != nil {
		t.Fatal(err)
	}
	v, err := conflict.FromEvent(first)
	if err != nil {
		t.Fatal(err)
	}
	first.SourceNodeID = "relay"
	first.TargetNodeID = "target"
	first.CreatedAt = time.Now()
	first.TraceID = "new-trace"
	data, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip event.SyncEvent
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatal(err)
	}
	w, err := conflict.FromEvent(roundtrip)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := conflict.Resolve(&v, w); err != nil || got != conflict.Duplicate {
		t.Fatalf("relay changed version: %s %v", got, err)
	}
	later, err := normalizer.New(normalizer.Options{NodeID: "edge", Now: func() time.Time { return time.Unix(9000, 0) }}).Normalize(change)
	if err != nil {
		t.Fatal(err)
	}
	u, err := conflict.FromEvent(later)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := conflict.Resolve(&v, u); err != nil || got != conflict.Duplicate {
		t.Fatalf("retry changed source time: %s %v", got, err)
	}
}

func TestProcessingFallbackCannotParticipateInLWW(t *testing.T) {
	evt, err := normalizer.New(normalizer.Options{NodeID: "edge"}).Normalize(cdc.ChangeEvent{DatabaseName: "db", TableName: "items", Operation: cdc.OperationInsert, PrimaryKey: map[string]any{"id": 1}, After: map[string]any{"id": 1}, BinlogFile: "mysql-bin.000001", BinlogPos: 1})
	if err != nil {
		t.Fatal(err)
	}
	if evt.Headers["event_time_source"] != "processing_fallback" {
		t.Fatal("fallback not marked")
	}
	if _, err := conflict.FromEvent(evt); err == nil {
		t.Fatal("processing time accepted as instruction time")
	}
}
