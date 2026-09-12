package conflict

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
)

func TestCanonicalNumericAndBinaryParts(t *testing.T) {
	for _, tc := range []struct {
		kind  string
		input any
		want  string
		fail  bool
	}{
		{"bigint unsigned", "18446744073709551615", "18446744073709551615", false},
		{"bigint unsigned", "18446744073709551616", "", true},
		{"bigint", "-9223372036854775808", "-9223372036854775808", false},
		{"bigint", "9223372036854775808", "", true},
		{"int", "001", "1", false}, {"int", json.Number("1e0"), "1", false},
		{"int", "1.0", "1", false}, {"int", "1.1", "", true},
		{"int", float64(1), "", true}, {"int", "1/1", "", true},
		{"tinyint", -128, "-128", false}, {"tinyint", 128, "", true},
		{"smallint unsigned", -1, "", true},
		{"decimal(30,10)", "12345678901234567890.1234567890", "123456789012345678901234567890", false},
		{"decimal(5,2)", "-0.00", "0", false}, {"decimal(5,2)", "1.234", "", true},
		{"decimal(5,2)", "1000", "", true}, {"decimal(5,2) unsigned", "-1", "", true},
		{"varbinary(3)", rowvalue.Binary{0, 128, 255}, string([]byte{0, 128, 255}), false},
		{"varbinary(3)", []byte{}, "", false}, {"binary(3)", []byte{0}, "", true},
		{"varbinary(3)", "AQID", "", true}, {"timestamp", "2026-09-11", "", true},
	} {
		t.Run(tc.kind+"_"+stringifyTestValue(tc.input), func(t *testing.T) {
			got, err := canonicalPart(context.Background(), nil, rulecheck.Column{Type: tc.kind}, tc.input)
			if (err != nil) != tc.fail || (!tc.fail && string(got) != tc.want) {
				t.Fatalf("got %q %v, want %q fail=%t", got, err, tc.want, tc.fail)
			}
		})
	}
}

func stringifyTestValue(value any) string { encoded, _ := json.Marshal(value); return string(encoded) }
