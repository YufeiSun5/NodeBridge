package rulecheck

import (
	"context"
	"errors"
	"strings"
)

// RequireNoTriggers fails closed when metadata visibility cannot be established.
func RequireNoTriggers(ctx context.Context, db SchemaReader, database, table string) error {
	var account string
	if err := db.QueryRowContext(ctx, "SELECT CURRENT_USER()").Scan(&account); err != nil {
		return err
	}
	at := strings.LastIndexByte(account, '@')
	if at < 1 {
		return errors.New("table_trigger_account_unverified")
	}
	grantee := "'" + strings.ReplaceAll(account[:at], "'", "''") + "'@'" + strings.ReplaceAll(account[at+1:], "'", "''") + "'"
	query := "SELECT COUNT(*) FROM (SELECT GRANTEE FROM information_schema.USER_PRIVILEGES WHERE PRIVILEGE_TYPE='TRIGGER' UNION ALL SELECT GRANTEE FROM information_schema.SCHEMA_PRIVILEGES WHERE PRIVILEGE_TYPE='TRIGGER' AND TABLE_SCHEMA=? UNION ALL SELECT GRANTEE FROM information_schema.TABLE_PRIVILEGES WHERE PRIVILEGE_TYPE='TRIGGER' AND TABLE_SCHEMA=? AND TABLE_NAME=?) AS grants_seen WHERE GRANTEE=?"
	var count int
	if err := db.QueryRowContext(ctx, query, database, database, table, grantee).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return errors.New("table_trigger_visibility_unverified")
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE EVENT_OBJECT_SCHEMA=? AND EVENT_OBJECT_TABLE=?", database, table).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return errors.New("table_triggers_unsupported")
	}
	return nil
}
