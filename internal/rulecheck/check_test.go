package rulecheck

import (
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestLocalSchemaSafety(t *testing.T) {
	base := rules.SyncRule{ID: "r", DatabaseName: "source", TableName: "items", Direction: rules.DirectionEdgeToServer, ConflictPolicy: rules.ConflictNone, Enable: true, PrimaryKeys: []string{"id"}, DeleteMode: rules.DeleteHard}
	baseSchema := Schema{Database: "target", Table: "items", Engine: "InnoDB", PrimaryKeys: []string{"id"}, Columns: []Column{{Name: "id", Type: "bigint"}, {Name: "name", Type: "varchar(64)"}, {Name: "source_name", Type: "varchar(64)"}}}
	for _, tc := range []struct {
		name, side, want string
		change           func(*rules.SyncRule, *Schema)
	}{
		{"valid", "target", "", func(*rules.SyncRule, *Schema) {}},
		{"partial_pk", "target", "primary_key_mismatch", func(_ *rules.SyncRule, s *Schema) { s.PrimaryKeys = []string{"id", "tenant_id"} }},
		{"wrong_engine", "target", "non_transactional_table", func(_ *rules.SyncRule, s *Schema) { s.Engine = "MyISAM" }},
		{"legacy_soft", "target", "soft_delete_column_missing", func(r *rules.SyncRule, _ *Schema) { r.DeleteMode = "" }},
		{"collision", "source", "column_mapping_collision", func(r *rules.SyncRule, _ *Schema) {
			r.ColumnMappings = []rules.ColumnMapping{{SourceColumn: "source_name", TargetColumn: "name"}}
		}},
		{"missing_include", "source", "source_column_missing", func(r *rules.SyncRule, _ *Schema) { r.IncludeColumns = []string{"missing"} }},
		{"missing_mapping", "target", "target_column_missing", func(r *rules.SyncRule, _ *Schema) {
			r.ColumnMappings = []rules.ColumnMapping{{SourceColumn: "source_name", TargetColumn: "missing"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, s := base, baseSchema
			tc.change(&r, &s)
			found := false
			for _, f := range ValidateLocal(r, s, tc.side) {
				if tc.want == "" && f.Severity == "error" {
					t.Fatalf("unexpected finding %+v", f)
				}
				if f.Code == tc.want {
					found = true
				}
			}
			if tc.want != "" && !found {
				t.Fatalf("missing finding %s", tc.want)
			}
		})
	}
}

func TestPermissionChecksCannotExecuteDML(t *testing.T) {
	for _, side := range []string{"source", "target"} {
		statements := permissionStatements(Schema{Database: "owned", Table: "rows"}, []string{"id", "value"}, side, rules.DeleteHard, rules.SyncModeOrderedCRUD)
		want := 4
		if side == "source" {
			want = 1
		}
		if len(statements) != want {
			t.Fatalf("statements=%v", statements)
		}
		for _, statement := range statements {
			if !strings.HasPrefix(statement.SQL, "EXPLAIN ") || strings.Contains(statement.SQL, "ANALYZE") || !strings.HasSuffix(statement.SQL, " WHERE 0") {
				t.Fatalf("unsafe check %s", statement.SQL)
			}
		}
	}
}

func TestSoftDeleteColumnTypes(t *testing.T) {
	rule := rules.SyncRule{PrimaryKeys: []string{"id"}, DeleteMode: rules.DeleteSoft}
	schema := Schema{Engine: "InnoDB", PrimaryKeys: []string{"id"}, Columns: []Column{{Name: "id", Type: "bigint"}, {Name: "is_deleted", Type: "tinyint"}, {Name: "deleted_at", Type: "datetime(3)"}, {Name: "deleted_by_node", Type: "varchar(64)"}, {Name: "updated_by_node", Type: "varchar(64)"}, {Name: "last_event_id", Type: "varchar(64)"}}}
	if findings := ValidateLocal(rule, schema, "target"); len(findings) != 0 {
		t.Fatal(findings)
	}
	schema.Columns[2].Type = "int"
	if findings := ValidateLocal(rule, schema, "target"); len(findings) != 1 || findings[0].Code != "soft_delete_column_type" {
		t.Fatal(findings)
	}
}
