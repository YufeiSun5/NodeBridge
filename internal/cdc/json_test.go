package cdc

import (
	"encoding/json"
	"testing"
)

func TestChangeJSONPreservesNumbers(t *testing.T) {
	var change ChangeEvent
	err := json.Unmarshal([]byte(`{"primary_key":{"id":9007199254740993},"before":{"value":-9223372036854775808},"after":{"value":999999999999.123456}}`), &change)
	if err != nil {
		t.Fatal(err)
	}
	for value, want := range map[any]string{change.PrimaryKey["id"]: "9007199254740993", change.Before["value"]: "-9223372036854775808", change.After["value"]: "999999999999.123456"} {
		n, ok := value.(json.Number)
		if !ok || n.String() != want {
			t.Fatalf("precision lost: %T %v", value, value)
		}
	}
	prior := ChangeEvent{TableName: "unchanged"}
	if err = json.Unmarshal([]byte(`{"table_name":"wrong","binlog_pos":-1}`), &prior); err == nil || prior.TableName != "unchanged" {
		t.Fatalf("invalid input mutated receiver: %+v %v", prior, err)
	}
}
