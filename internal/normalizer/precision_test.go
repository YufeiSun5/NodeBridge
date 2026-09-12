package normalizer

import (
	"encoding/json"
	"testing"
)

func TestVersionFromCanalStringAndJSONNumber(t *testing.T) {
	for _, value := range []any{"9007199254740993", json.Number("9007199254740993"), int64(9007199254740993)} {
		if got := int64Value(map[string]any{"sync_version": value}, "sync_version"); got != 9007199254740993 {
			t.Fatalf("%T: %d", value, got)
		}
	}
}
