package rulecheck

import (
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestTypeCompatibilityPreservesRangeAndPrecision(t *testing.T) {
	for _, tc := range []struct {
		source, target string
		want           bool
	}{
		{"binary(8)", "binary(16)", false}, {"binary(16)", "binary(8)", false}, {"binary(8)", "binary(8)", true}, {"varbinary(8)", "varbinary(16)", true},
		{"int(11)", "bigint", true}, {"bigint", "int", false}, {"int unsigned", "bigint", true}, {"bigint unsigned", "bigint", false}, {"int", "bigint unsigned", false},
		{"decimal(10,3)", "decimal(12,4)", true}, {"decimal(10,3)", "decimal(10,4)", false}, {"decimal(10,3)", "decimal(12,2)", false},
		{"varchar(40)", "varchar(80)", true}, {"varchar(80)", "varchar(40)", false}, {"datetime(3)", "datetime(6)", true}, {"datetime(6)", "datetime(3)", false}, {"datetime(3)", "timestamp(3)", false},
	} {
		if got := typeFits(tc.source, tc.target); got != tc.want {
			t.Errorf("%s -> %s got=%t", tc.source, tc.target, got)
		}
	}
}

func TestPairRejectsRequiredMissingNullableAndNarrowColumns(t *testing.T) {
	rule := rules.SyncRule{PrimaryKeys: []string{"id"}, ColumnMappings: []rules.ColumnMapping{{SourceColumn: "label", TargetColumn: "name"}}}
	source := Schema{Columns: []Column{{Name: "id", Type: "bigint"}, {Name: "label", Type: "varchar(80)", Nullable: true}}}
	target := Schema{Columns: []Column{{Name: "id", Type: "bigint"}, {Name: "name", Type: "varchar(40)"}, {Name: "required_local", Type: "int"}}}
	findings := ValidatePair(rule, source, target)
	for _, code := range []string{"nullable_source_to_required_target", "column_type_incompatible", "unmapped_required_target_column"} {
		found := false
		for _, finding := range findings {
			if finding.Code == code {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s: %+v", code, findings)
		}
	}
	target.Columns[1].Nullable = true
	target.Columns[1].Type = "varchar(100)"
	target.Columns[2].HasDefault = true
	if findings := ValidatePair(rule, source, target); len(findings) != 0 {
		t.Fatal(findings)
	}
}
