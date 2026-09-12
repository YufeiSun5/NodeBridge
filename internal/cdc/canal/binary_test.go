package canal

import (
	"reflect"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
	withlinentry "github.com/withlin/canal-go/protocol/entry"
)

func TestBinaryColumnTypes(t *testing.T) {
	for _, sqlType := range []int32{-2, -3, -4, 2004} {
		column := &withlinentry.Column{Name: "raw", SqlType: sqlType, Value: "\x00\u0080\u00ff"}
		got, err := columnValue(column)
		if err != nil || !reflect.DeepEqual(got, rowvalue.Binary{0, 128, 255}) {
			t.Fatal(sqlType, got, err)
		}
		column.IsNullPresent = &withlinentry.Column_IsNull{IsNull: true}
		if got, err = columnValue(column); err != nil || got != nil {
			t.Fatal("NULL binary", got, err)
		}
		column.IsNullPresent, column.Value = nil, "\u0100"
		if _, err := columnsToMap([]*withlinentry.Column{column}); err == nil {
			t.Fatal("invalid binary accepted")
		}
	}
	for _, sqlType := range []int32{12, 2005, 3, -5} {
		column := &withlinentry.Column{SqlType: sqlType, Value: "\u0080\u00ff"}
		got, err := columnValue(column)
		if err != nil || got != column.Value {
			t.Fatal("text/numeric value changed", got, err)
		}
	}
}
