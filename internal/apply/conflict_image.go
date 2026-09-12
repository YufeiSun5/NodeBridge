package apply

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
)

func readConflictImage(ctx context.Context, tx *sql.Tx, schema rulecheck.Schema, mapped mapper.MappedEvent) (map[string]any, error) {
	var columns []rulecheck.Column
	var projections []string
	for _, column := range schema.Columns {
		if column.Generated() {
			continue
		}
		columns = append(columns, column)
		name := quoteIdentifier(column.Name)
		if column.Collation != "" {
			name = "CONVERT(" + name + " USING utf8mb4)"
		}
		projections = append(projections, "CAST("+name+" AS BINARY)")
	}
	where, args, err := wherePrimaryKey(mapped.TargetPrimaryKey)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT "+strings.Join(projections, ",")+" FROM "+qualifiedTable(schema.Database, schema.Table)+" WHERE "+where+" LIMIT 2", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, errors.New("conflict_winner_row_missing")
	}
	values := make([][]byte, len(columns))
	dest := make([]any, len(columns))
	for i := range dest {
		dest[i] = &values[i]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	result := make(map[string]any, len(columns))
	for i, column := range columns {
		if values[i] == nil {
			result[column.Name] = nil
			continue
		}
		kind := strings.ToLower(column.Type)
		binary := strings.Contains(kind, "blob") || strings.HasPrefix(kind, "binary") || strings.HasPrefix(kind, "varbinary") || strings.HasPrefix(kind, "bit(")
		if binary {
			result[column.Name] = rowvalue.Binary(values[i])
		} else {
			result[column.Name] = string(values[i])
		}
	}
	if rows.Next() {
		return nil, errors.New("conflict_winner_row_not_unique")
	}
	return result, rows.Err()
}
