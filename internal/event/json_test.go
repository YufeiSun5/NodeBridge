package event

import (
	"encoding/json"
	"testing"
)

func TestSyncEventDecodesExactNumericValues(t *testing.T) {
	const body = `{"event_id":"precise","primary_key":{"id":18446744073709551615},"before":{"n":-9223372036854775808},"after":{"i":9007199254740993,"d":999999999999.123456,"nested":[1.000000000000000001,null,"9007199254740993"]},"sync_version":9223372036854775807}`
	var evt SyncEvent
	if err := json.Unmarshal([]byte(body), &evt); err != nil {
		t.Fatal(err)
	}
	for label, pair := range map[string][2]any{
		"pk":      {evt.PrimaryKey["id"], "18446744073709551615"},
		"before":  {evt.Before["n"], "-9223372036854775808"},
		"integer": {evt.After["i"], "9007199254740993"},
		"decimal": {evt.After["d"], "999999999999.123456"},
		"nested":  {evt.After["nested"].([]any)[0], "1.000000000000000001"},
	} {
		n, ok := pair[0].(json.Number)
		if !ok || n.String() != pair[1] {
			t.Fatalf("%s: %#v", label, pair[0])
		}
	}
	encoded, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip SyncEvent
	if err = json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.After["i"] != evt.After["i"] || roundTrip.After["d"] != evt.After["d"] {
		t.Fatal("numeric round trip changed values")
	}
}

func TestSyncEventRejectsMalformedJSONWithoutMutatingReceiver(t *testing.T) {
	for _, body := range []string{`{"after":{"n":1e}}`, `{"sync_version":9223372036854775808}`, `{"after":{}} {}`} {
		evt := SyncEvent{EventID: "original"}
		if err := json.Unmarshal([]byte(body), &evt); err == nil {
			t.Fatalf("accepted %s", body)
		}
		if evt.EventID != "original" {
			t.Fatal("mutated on decode error")
		}
	}
}
