package rulecheck

import (
	"context"
	"database/sql"
	"fmt"
)

// Metadata can be privilege-filtered; an empty result is not proof of absence.
func checkDependencies(ctx context.Context, db *sql.DB, schema Schema, result *Result) {
	checks := []struct {
		code, query, message string
		args                 []any
	}{
		{"table_triggers", "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE EVENT_OBJECT_SCHEMA=? AND EVENT_OBJECT_TABLE=?", "visible triggers require side-effect and definer-permission review", []any{schema.Database, schema.Table}},
		{"outbound_foreign_keys", "SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=? AND TABLE_NAME=?", "visible foreign keys require parent-row availability and load-order checks", []any{schema.Database, schema.Table}},
		{"inbound_foreign_keys", "SELECT COUNT(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE UNIQUE_CONSTRAINT_SCHEMA=? AND REFERENCED_TABLE_NAME=?", "visible referencing foreign keys may reject or cascade updates/deletes", []any{schema.Database, schema.Table}},
	}
	for _, check := range checks {
		var count int
		if err := db.QueryRowContext(ctx, check.query, check.args...).Scan(&count); err != nil {
			result.add(check.code+"_unavailable", "warning", err.Error())
			continue
		}
		if count > 0 {
			result.add(check.code, "warning", fmt.Sprintf("%d %s", count, check.message))
		}
	}
}
