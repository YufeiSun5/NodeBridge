package apply

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
)

func stampDeleteReplay(ctx context.Context, tx *sql.Tx, mapped mapper.MappedEvent) error {
	last, by := mapped.TargetColumn("last_event_id"), mapped.TargetColumn("updated_by_node")
	_, hasLast := mapped.TargetBefore[last]
	_, hasBy := mapped.TargetBefore[by]
	if !hasLast || !hasBy || mapped.Event.OriginNodeID == "" {
		return errors.New("delete_replay_full_metadata_required")
	}
	// Acquire table metadata and row locks before examining trigger visibility.
	where, args, err := wherePrimaryKey(mapped.TargetPrimaryKey)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT 1 FROM "+qualifiedTable(mapped.TargetDatabase, mapped.TargetTable)+" WHERE "+where+" LIMIT 2 FOR UPDATE", args...)
	if err != nil {
		return err
	}
	count := 0
	for rows.Next() {
		count++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if count != 1 {
		return errors.New("multiple_target_rows: delete replay key is not unique")
	}
	if err := rulecheck.RequireNoTriggers(ctx, tx, mapped.TargetDatabase, mapped.TargetTable); err != nil {
		return fmt.Errorf("delete_replay_trigger_check: %w", err)
	}
	stampArgs := []any{mapped.Event.EventID, mapped.Event.OriginNodeID}
	stampArgs = append(stampArgs, args...)
	if _, err := tx.ExecContext(ctx, "UPDATE "+qualifiedTable(mapped.TargetDatabase, mapped.TargetTable)+" SET "+quoteIdentifier(last)+"=?, "+quoteIdentifier(by)+"=? WHERE "+where, stampArgs...); err != nil {
		return err
	}
	// No committed live row can retain this marker: deletion and receipt are atomic.
	_, err = tx.ExecContext(ctx, "INSERT INTO sync_delete_replay (event_id,origin_node_id,database_name,table_name) VALUES (?,?,?,?)", mapped.Event.EventID, mapped.Event.OriginNodeID, mapped.TargetDatabase, mapped.TargetTable)
	return err
}
