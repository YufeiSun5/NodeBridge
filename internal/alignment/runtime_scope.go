package alignment

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
)

// TableScope is a local physical table read or written by an enabled rule.
// Rule IDs and direction changes cannot bypass a fence on the same table.
type TableScope struct{ Database, Table string }

func LoadActiveCutoversForTables(ctx context.Context, db *sql.DB, tables []TableScope) ([]CutoverProof, error) {
	if len(tables) == 0 {
		return nil, nil
	}
	var folding int
	if err := db.QueryRowContext(ctx, "SELECT @@lower_case_table_names").Scan(&folding); err != nil {
		return nil, err
	}
	if folding < 0 || folding > 2 {
		return nil, errors.New("alignment_table_case_mode_invalid")
	}
	seen := map[string]bool{}
	for _, table := range tables {
		if err := validateTable(table.Database, table.Table); err != nil {
			return nil, err
		}
		seen[canonicalJobScope(table.Database, table.Table, folding)] = true
	}
	scopes := make([]string, 0, len(seen))
	for scope := range seen {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return loadActiveCutovers(ctx, db, scopes, folding)
}

func scopePredicate(scopes []string) (string, []any) {
	if len(scopes) == 0 {
		return "", nil
	}
	args := make([]any, len(scopes))
	for i, scope := range scopes {
		args[i] = scope
	}
	return " AND scope_hash IN (" + strings.TrimSuffix(strings.Repeat("?,", len(scopes)), ",") + ")", args
}

func scopeIncluded(scopes []string, scope string) bool {
	for _, candidate := range scopes {
		if candidate == scope {
			return true
		}
	}
	return false
}
