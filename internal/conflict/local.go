package conflict

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type LocalRecorder struct {
	DB     *sql.DB
	NodeID string
	Rules  *rules.RuleSet
}

// RecordLocal registers an already committed source write, never rewrites it.
// It must run after replay filtering and before publishing or releasing a fence.
func (r LocalRecorder) RecordLocal(ctx context.Context, evt event.SyncEvent) error {
	if r.DB == nil || r.NodeID == "" || r.Rules == nil {
		return errors.New("conflict_local_recorder_dependencies_required")
	}
	rule := r.Rules.FindForNode(evt.DatabaseName, evt.TableName, r.NodeID, r.NodeID)
	if rule == nil || !rule.Enable || rule.Direction != rules.DirectionBidirectional || rule.ConflictPolicy != rules.ConflictLastWriteWin {
		return nil
	}
	if evt.OriginNodeID != r.NodeID || evt.SourceNodeID != r.NodeID {
		return errors.New("conflict_local_origin_required")
	}
	version, err := FromEvent(evt)
	if err != nil {
		return err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	schema, err := rulecheck.ReadSchema(ctx, tx, evt.DatabaseName, evt.TableName)
	if err != nil {
		return err
	}
	key, err := CanonicalKey(ctx, tx, schema, evt.PrimaryKey)
	if err != nil {
		return err
	}
	if evt.EventType == event.TypeUpdate {
		before := make(map[string]any, len(schema.PrimaryKeys))
		for _, name := range schema.PrimaryKeys {
			before[name] = evt.Before[name]
		}
		previous, err := CanonicalKey(ctx, tx, schema, before)
		if err != nil {
			return err
		}
		if !bytes.Equal(previous.CanonicalKey, key.CanonicalKey) {
			return errors.New("conflict_primary_key_change_unsupported")
		}
	}
	if err := PinKeySchema(ctx, tx, schema); err != nil {
		return err
	}
	decision, err := ApplyInTx(ctx, tx, key, version, func(context.Context, *sql.Tx) error { return nil }, func(context.Context, *sql.Tx, string) error { return nil })
	if err != nil {
		return err
	}
	if decision == Apply {
		state := RowState{Version: version, PrimaryKey: evt.PrimaryKey, DeleteMode: rules.DeleteHard, Columns: map[string]string{}}
		for _, marker := range []string{"last_event_id", "updated_by_node"} {
			column := marker
			for _, mapping := range rule.ColumnMappings {
				if mapping.TargetColumn == marker {
					column = mapping.SourceColumn
					break
				}
			}
			state.Columns[marker] = column
		}
		if !version.Deleted {
			state.Row = make(map[string]any)
			for _, column := range schema.Columns {
				if column.Generated() {
					continue
				}
				value, ok := evt.After[column.Name]
				if !ok {
					return fmt.Errorf("conflict_full_source_image_required: %s", column.Name)
				}
				state.Row[column.Name] = value
			}
		}
		if err := SaveWinnerState(ctx, tx, key, state); err != nil {
			return err
		}
	} else if decision == Superseded {
		// Capture must keep advancing so repair can fence all later local writes.
		if err := SetRepairRequired(ctx, tx, key, true); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit local conflict version (outcome may be unknown): %w", err)
	}
	return nil
}
