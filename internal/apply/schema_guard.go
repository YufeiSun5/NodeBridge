package apply

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

// NewCheckedSQLWorker is used by production entry points; metadata is checked
// while the same transaction holds the business table's metadata lock.
func NewCheckedSQLWorker(db *sql.DB) *SQLWorker {
	w := NewSQLWorker(db)
	w.CheckSchema = true
	return w
}

func checkTargetSchemas(ctx context.Context, tx *sql.Tx, events []mapper.MappedEvent) error {
	checked := map[string]bool{}
	for _, evt := range events {
		keys := sortedKeys(evt.TargetPrimaryKey)
		sort.Strings(keys)
		soft := evt.Event.EventType == event.TypeDelete && evt.DeleteMode != rules.DeleteHard
		key := evt.TargetDatabase + "." + evt.TargetTable + "|" + strings.Join(keys, ",") + fmt.Sprint(soft)
		if !soft && checked[key] {
			continue
		}
		rows, err := tx.QueryContext(ctx, "SELECT "+quoteJoin(keys)+" FROM "+qualifiedTable(evt.TargetDatabase, evt.TargetTable)+" LIMIT 0")
		if err != nil {
			return fmt.Errorf("target_schema_unavailable: %w", err)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		schema, err := rulecheck.ReadSchema(ctx, tx, evt.TargetDatabase, evt.TargetTable)
		if err != nil {
			return err
		}
		rule := rules.SyncRule{PrimaryKeys: keys, DeleteMode: rules.DeleteHard}
		if soft {
			rule.DeleteMode = rules.DeleteSoft
			for _, source := range []string{"is_deleted", "deleted_at", "deleted_by_node", "updated_by_node", "last_event_id"} {
				if target := evt.TargetColumns[source]; target != "" {
					rule.ColumnMappings = append(rule.ColumnMappings, rules.ColumnMapping{SourceColumn: source, TargetColumn: target})
				}
			}
		}
		for _, finding := range rulecheck.ValidateLocal(rule, schema, "target") {
			if finding.Severity == "error" {
				return fmt.Errorf("%s: %s.%s: %s", finding.Code, evt.TargetDatabase, evt.TargetTable, finding.Message)
			}
		}
		checked[key] = true
	}
	return nil
}
