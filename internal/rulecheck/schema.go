package rulecheck

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/mapper"
)

type Column struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	HasDefault bool   `json:"has_default"`
	Extra      string `json:"extra"`
	Collation  string `json:"collation,omitempty"`
}

type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

type Schema struct {
	Database      string   `json:"database"`
	Table         string   `json:"table"`
	Engine        string   `json:"engine"`
	Columns       []Column `json:"columns"`
	PrimaryKeys   []string `json:"primary_keys"`
	UniqueIndexes []Index  `json:"unique_indexes"`
}

type SchemaReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func ReadSchema(ctx context.Context, db SchemaReader, database, table string) (Schema, error) {
	s := Schema{Database: database, Table: table, Columns: []Column{}, PrimaryKeys: []string{}, UniqueIndexes: []Index{}}
	if err := mapper.ValidateIdentifier(database); err != nil {
		return s, err
	}
	if err := mapper.ValidateIdentifier(table); err != nil {
		return s, err
	}
	if err := db.QueryRowContext(ctx, "SELECT COALESCE(ENGINE, '') FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?", database, table).Scan(&s.Engine); err != nil {
		return s, fmt.Errorf("table_missing_or_inaccessible: %s.%s: %w", database, table, err)
	}
	rows, err := db.QueryContext(ctx, "SELECT COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,COLUMN_DEFAULT,EXTRA,COALESCE(COLLATION_NAME,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? ORDER BY ORDINAL_POSITION", database, table)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var c Column
		var nullable string
		var defaultValue sql.NullString
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &defaultValue, &c.Extra, &c.Collation); err != nil {
			rows.Close()
			return s, err
		}
		c.Nullable, c.HasDefault = nullable == "YES", defaultValue.Valid
		s.Columns = append(s.Columns, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return s, err
	}
	rows, err = db.QueryContext(ctx, "SELECT INDEX_NAME,COALESCE(COLUMN_NAME,'') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND NON_UNIQUE=0 ORDER BY INDEX_NAME,SEQ_IN_INDEX", database, table)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, column string
		if err := rows.Scan(&name, &column); err != nil {
			return s, err
		}
		if name == "PRIMARY" {
			s.PrimaryKeys = append(s.PrimaryKeys, column)
		}
		if len(s.UniqueIndexes) == 0 || s.UniqueIndexes[len(s.UniqueIndexes)-1].Name != name {
			s.UniqueIndexes = append(s.UniqueIndexes, Index{Name: name, Columns: []string{}})
		}
		last := &s.UniqueIndexes[len(s.UniqueIndexes)-1]
		last.Columns = append(last.Columns, column)
	}
	return s, rows.Err()
}

func (c Column) Generated() bool {
	return strings.Contains(strings.ToUpper(c.Extra), "GENERATED") && !strings.Contains(strings.ToUpper(c.Extra), "DEFAULT_GENERATED")
}

func quoted(name string) string { return "`" + name + "`" }

func qualified(s Schema) string { return quoted(s.Database) + "." + quoted(s.Table) }
