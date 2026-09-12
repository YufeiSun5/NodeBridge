package conflict

import (
	"encoding/json"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestWinnerImageRoundTripKeepsExactValues(t *testing.T) {
	state := RowState{Version: version("node", "event", 1000000, false), PrimaryKey: map[string]any{"id": uint64(18446744073709551615)}, Row: map[string]any{"raw": rowvalue.Binary{0, 128, 255}, "amount": "12345678901234567890.1234567890", "null": nil, "empty": ""}, DeleteMode: rules.DeleteHard}
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeState(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.PrimaryKey["id"] != json.Number("18446744073709551615") || decoded.Row["amount"] != state.Row["amount"] || decoded.Row["null"] != nil || decoded.Row["empty"] != "" {
		t.Fatal("winner image lost precision or nullness")
	}
	if raw, ok := decoded.Row["raw"].(rowvalue.Binary); !ok || string(raw) != string([]byte{0, 128, 255}) {
		t.Fatalf("binary lost: %v", decoded.Row["raw"])
	}
}
