package syncruntime

import (
	"encoding/json"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/mapper"
)

func TestIndependentUpdateKeyStaysConservative(t *testing.T) {
	for _, value := range []any{"A", "a", "01", "+1", "-0", "1.0", "1 ", nil, "\uFF11"} {
		if key, ok := independentUpdateKey(mapper.MappedEvent{TargetPrimaryKey: map[string]any{"id": value}}); ok {
			t.Fatalf("ambiguous key batched: %v (%s)", value, key)
		}
	}
	for _, value := range []any{"0", "1", "-1", int64(9007199254740993), json.Number("18446744073709551615")} {
		if _, ok := independentUpdateKey(mapper.MappedEvent{TargetPrimaryKey: map[string]any{"id": value}}); !ok {
			t.Fatalf("canonical integer not batched: %v", value)
		}
	}
	for _, mapped := range []mapper.MappedEvent{
		{TargetPrimaryKey: map[string]any{"id": "2"}, TargetBefore: map[string]any{"id": "1"}},
		{TargetPrimaryKey: map[string]any{"id": "1"}, TargetAfter: map[string]any{"id": "2"}},
	} {
		if _, ok := independentUpdateKey(mapped); ok {
			t.Fatal("primary-key mutation batched")
		}
	}
}
