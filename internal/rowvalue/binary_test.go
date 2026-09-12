package rowvalue_test

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/normalizer"
	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestBinaryCDCWireReplayMappingAndSQL(t *testing.T) {
	want := []byte{0, 1, 127, 128, 255}
	binary, err := rowvalue.FromCanal("\x00\x01\x7f\u0080\u00ff")
	if err != nil || !bytes.Equal(binary, want) {
		t.Fatal(binary, err)
	}
	change := cdc.ChangeEvent{DatabaseName: "source_db", TableName: "source_table", Operation: cdc.OperationInsert,
		PrimaryKey: map[string]any{"key": binary}, After: map[string]any{"key": binary, "empty": rowvalue.Binary{}, "null": nil, "text": "\u0080\u00ff", "decimal": json.Number("12345678901234567890.123456")}}
	data, err := json.Marshal(change)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &change); err != nil {
		t.Fatal(err)
	}
	evt, err := normalizer.New(normalizer.Options{NodeID: "owned"}).Normalize(change)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		data, err = json.Marshal(evt)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &evt); err != nil {
			t.Fatal(err)
		}
	}
	mapped, err := mapper.MapEvent(evt, rules.SyncRule{PrimaryKeys: []string{"key"}, TargetPrimaryKeys: []string{"target_key"}, TargetDatabaseName: "target_db", TargetTableName: "target_table"})
	if err != nil {
		t.Fatal(err)
	}
	value, err := driver.DefaultParameterConverter.ConvertValue(mapped.TargetPrimaryKey["target_key"])
	if err != nil || !bytes.Equal(value.([]byte), want) {
		t.Fatal(value, err)
	}
	if _, ok := evt.PrimaryKey["key"]; !ok {
		t.Fatal("source key changed")
	}
	if !reflect.DeepEqual(mapped.TargetAfter["target_key"], evt.PrimaryKey["key"]) {
		t.Fatal("binary key/image mismatch")
	}
	if mapped.TargetAfter["null"] != nil || mapped.TargetAfter["text"] != "\u0080\u00ff" {
		t.Fatal("NULL/text changed")
	}
	if mapped.TargetAfter["decimal"] != json.Number("12345678901234567890.123456") {
		t.Fatal("decimal changed")
	}
	empty, err := driver.DefaultParameterConverter.ConvertValue(mapped.TargetAfter["empty"])
	if err != nil || empty == nil || len(empty.([]byte)) != 0 {
		t.Fatal("empty binary became NULL", empty, err)
	}
}

func TestBinaryRejectsMalformedEnvelope(t *testing.T) {
	for _, value := range []string{`{"$nodebridge_binary_base64":1}`, `{"$nodebridge_binary_base64":"!"}`, `{"$nodebridge_binary_base64":"AA==","extra":true}`} {
		var evt event.SyncEvent
		if err := json.Unmarshal([]byte(`{"after":{"bytes":`+value+`}}`), &evt); err == nil {
			t.Fatal("accepted", value)
		}
	}
	if _, err := rowvalue.FromCanal("\u0100"); err == nil {
		t.Fatal("accepted invalid Latin-1")
	}
}
