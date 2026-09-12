package canal

import "testing"

func TestParseAlterColumnSQLAdd(t *testing.T) {
	change, recognized, err := ParseAlterColumnSQL("scada_edge", "device_config", "ALTER TABLE `scada_edge`.`device_config` ADD COLUMN `governed_note` VARCHAR(64) NOT NULL DEFAULT 'new value' COMMENT 'MCP governed'")
	if err != nil {
		t.Fatalf("ParseAlterColumnSQL returned error: %v", err)
	}
	if !recognized || change == nil || change.Operation != "ADD_COLUMN" {
		t.Fatalf("unexpected change %+v recognized=%t", change, recognized)
	}
	column := change.Column
	if column.Name != "governed_note" || column.Type != "VARCHAR(64)" || column.Nullable || column.Default.Mode != "literal" || column.Default.Value != "new value" || column.Comment != "MCP governed" {
		t.Fatalf("unexpected column %+v", column)
	}
}

func TestParseAlterColumnSQLDrop(t *testing.T) {
	change, recognized, err := ParseAlterColumnSQL("scada_edge", "device_config", "ALTER TABLE device_config DROP COLUMN governed_note;")
	if err != nil || !recognized || change.Operation != "DROP_COLUMN" || change.Column.Name != "governed_note" {
		t.Fatalf("unexpected change=%+v recognized=%t err=%v", change, recognized, err)
	}
}

func TestParseAlterColumnSQLSkipsCreateAndOtherAlterOperations(t *testing.T) {
	for _, sql := range []string{
		"CREATE TABLE device_config (id BIGINT)",
		"ALTER TABLE device_config MODIFY COLUMN value VARCHAR(200)",
		"ALTER TABLE device_config ADD INDEX idx_value (value)",
	} {
		change, recognized, err := ParseAlterColumnSQL("scada_edge", "device_config", sql)
		if err != nil || recognized || change != nil {
			t.Fatalf("expected unsupported DDL to be skipped, sql=%q change=%+v recognized=%t err=%v", sql, change, recognized, err)
		}
	}
}

func TestParseAlterColumnSQLRejectsCompositeOrUnsafeAdd(t *testing.T) {
	for _, sql := range []string{
		"ALTER TABLE device_config ADD COLUMN a INT, ADD COLUMN b INT",
		"ALTER TABLE device_config ADD COLUMN payload VARCHAR(32) GENERATED ALWAYS AS (name)",
		"ALTER TABLE device_config ADD COLUMN payload VARCHAR(32); DROP TABLE x",
	} {
		if _, recognized, err := ParseAlterColumnSQL("scada_edge", "device_config", sql); !recognized || err == nil {
			t.Fatalf("expected recognized unsafe DDL rejection for %q, recognized=%t err=%v", sql, recognized, err)
		}
	}
}

func TestParseAlterColumnSQLRejectsHeaderMismatch(t *testing.T) {
	if _, recognized, err := ParseAlterColumnSQL("scada_edge", "device_config", "ALTER TABLE other_table ADD COLUMN note TEXT"); !recognized || err == nil {
		t.Fatalf("expected header mismatch, recognized=%t err=%v", recognized, err)
	}
}

func TestParseAlterColumnSQLSupportsNumericAndTimestampDefaults(t *testing.T) {
	change, _, err := ParseAlterColumnSQL("scada_edge", "device_config", "ALTER TABLE device_config ADD COLUMN retry_count INT UNSIGNED NOT NULL DEFAULT 0")
	if err != nil || change.Column.Type != "INT UNSIGNED" || change.Column.Default.Mode != "literal" {
		t.Fatalf("unexpected numeric change=%+v err=%v", change, err)
	}
	change, _, err = ParseAlterColumnSQL("scada_edge", "device_config", "ALTER TABLE device_config ADD COLUMN observed_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)")
	if err != nil || change.Column.Default.Mode != "current_timestamp" || change.Column.Default.Value != 3 {
		t.Fatalf("unexpected timestamp change=%+v err=%v", change, err)
	}
}
