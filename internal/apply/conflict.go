package apply

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/conflict"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type CaptureFence interface{ Wait(context.Context) error }

func containsConflictEvent(events []mapper.MappedEvent) bool {
	for _, evt := range events {
		if evt.ConflictPolicy == rules.ConflictLastWriteWin {
			return true
		}
	}
	return false
}

func (w *SQLWorker) applyConflict(ctx context.Context, mapped mapper.MappedEvent) (Result, error) {
	if w.CaptureFence == nil {
		return Result{}, errors.New("conflict_local_capture_fence_required")
	}
	if mapped.SyncMode == rules.SyncModeAppendOnly {
		return Result{}, errors.New("conflict_append_only_unsupported")
	}
	var original event.SyncEvent
	decoder := json.NewDecoder(bytes.NewReader(mapped.ConflictSource))
	decoder.UseNumber()
	if err := decoder.Decode(&original); err != nil {
		return Result{}, fmt.Errorf("conflict_original_event_required: %w", err)
	}
	if original.EventID != mapped.Event.EventID || original.OriginNodeID != mapped.Event.OriginNodeID || original.EventType != mapped.Event.EventType || original.DatabaseName != mapped.SourceDatabase || original.TableName != mapped.SourceTable {
		return Result{}, errors.New("conflict_original_event_mismatch")
	}
	if err := validateConflictProjection(original, mapped); err != nil {
		return Result{}, err
	}
	version, err := conflict.FromEvent(original)
	if err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	tx, err := w.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err := checkTargetSchemas(ctx, tx, []mapper.MappedEvent{mapped}); err != nil {
		return Result{}, err
	}
	schema, err := rulecheck.ReadSchema(ctx, tx, mapped.TargetDatabase, mapped.TargetTable)
	if err != nil {
		return Result{}, err
	}
	if err := validateConflictTarget(ctx, tx, schema, mapped); err != nil {
		return Result{}, err
	}
	key, err := conflict.CanonicalKey(ctx, tx, schema, mapped.TargetPrimaryKey)
	if err != nil {
		return Result{}, err
	}
	exists, err := lockConflictRow(ctx, tx, mapped)
	if err != nil {
		return Result{}, err
	}
	// No version, receipt or schema-registry locks may precede this fresh fence.
	if err := w.CaptureFence.Wait(ctx); err != nil {
		return Result{}, fmt.Errorf("fence local capture: %w", err)
	}
	if err := conflict.PinKeySchema(ctx, tx, schema); err != nil {
		return Result{}, err
	}
	decision, err := conflict.ApplyInTx(ctx, tx, key, version, func(ctx context.Context, tx *sql.Tx) error {
		switch mapped.Event.EventType {
		case event.TypeInsert, event.TypeUpdate:
			if exists {
				return applyUpdate(ctx, tx, mapped)
			}
			return applyInsert(ctx, tx, mapped)
		case event.TypeDelete:
			if mapped.DeleteMode == rules.DeleteHard {
				if !exists {
					return nil
				}
				mapped.TrackDeleteReplay = true
				return applyHardDelete(ctx, tx, mapped)
			}
			if !exists {
				soft := mapped
				soft.TargetAfter = make(map[string]any, len(mapped.TargetBefore))
				for column, value := range mapped.TargetBefore {
					soft.TargetAfter[column] = value
				}
				soft.TargetAfter[mapped.TargetColumn("is_deleted")] = 1
				soft.TargetAfter[mapped.TargetColumn("deleted_at")] = version.Time
				soft.TargetAfter[mapped.TargetColumn("deleted_by_node")] = mapped.Event.OriginNodeID
				return applyInsert(ctx, tx, soft)
			}
			return applySoftDelete(ctx, tx, mapped, version.Time)
		}
		return errors.New("conflict_unsupported_event_type")
	}, func(ctx context.Context, tx *sql.Tx, decision string) error {
		applied, err := alreadyApplied(ctx, tx, mapped.Event.EventID)
		if err != nil {
			return err
		}
		if applied {
			if decision != conflict.Duplicate {
				return errors.New("conflict_apply_receipt_without_version")
			}
			return nil
		}
		return insertApplyLog(ctx, tx, mapped, w.Clock())
	})
	if err != nil {
		return Result{}, err
	}
	if decision == conflict.Apply {
		state := conflict.RowState{Version: version, PrimaryKey: mapped.TargetPrimaryKey, Columns: mapped.TargetColumns, DeleteMode: mapped.DeleteMode}
		if !version.Deleted || mapped.DeleteMode != rules.DeleteHard {
			state.Row, err = readConflictImage(ctx, tx, schema, mapped)
			if err != nil {
				return Result{}, err
			}
		}
		if err := conflict.SaveWinnerState(ctx, tx, key, state); err != nil {
			return Result{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit conflict apply (outcome may be unknown): %w", err)
	}
	return Result{EventID: mapped.Event.EventID, SourceTable: mapped.SourceTable, TargetTable: mapped.TargetTable, AlreadyApplied: decision == conflict.Duplicate, ConflictDecision: decision}, nil
}

func validateConflictProjection(original event.SyncEvent, mapped mapper.MappedEvent) error {
	for _, pair := range []struct{ source, target map[string]any }{{original.PrimaryKey, mapped.TargetPrimaryKey}, {original.Before, mapped.TargetBefore}, {original.After, mapped.TargetAfter}} {
		for target, value := range pair.target {
			source := target
			for from, to := range mapped.TargetColumns {
				if to == target {
					source = from
					break
				}
			}
			originalValue, ok := pair.source[source]
			if !ok {
				return fmt.Errorf("conflict_projection_mismatch: %s", target)
			}
			left, err := json.Marshal(originalValue)
			if err != nil {
				return err
			}
			right, err := json.Marshal(value)
			if err != nil {
				return err
			}
			if !bytes.Equal(left, right) {
				return fmt.Errorf("conflict_projection_mismatch: %s", target)
			}
		}
	}
	return nil
}

func lockConflictRow(ctx context.Context, tx *sql.Tx, mapped mapper.MappedEvent) (bool, error) {
	where, args, err := wherePrimaryKey(mapped.TargetPrimaryKey)
	if err != nil {
		return false, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT 1 FROM "+qualifiedTable(mapped.TargetDatabase, mapped.TargetTable)+" FORCE INDEX (PRIMARY) WHERE "+where+" LIMIT 2 FOR UPDATE", args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if count > 1 {
		return false, errors.New("multiple_target_rows: conflict key is not unique")
	}
	return count == 1, nil
}

func validateConflictTarget(ctx context.Context, tx *sql.Tx, schema rulecheck.Schema, mapped mapper.MappedEvent) error {
	if err := rulecheck.RequireNoTriggers(ctx, tx, schema.Database, schema.Table); err != nil {
		return err
	}
	var foreignKeys int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE REFERENCED_TABLE_NAME IS NOT NULL AND ((TABLE_SCHEMA=? AND TABLE_NAME=?) OR (REFERENCED_TABLE_SCHEMA=? AND REFERENCED_TABLE_NAME=?))", schema.Database, schema.Table, schema.Database, schema.Table).Scan(&foreignKeys); err != nil {
		return err
	}
	if foreignKeys != 0 {
		return errors.New("conflict_foreign_keys_unsupported")
	}
	image := mapped.TargetAfter
	if mapped.Event.EventType == event.TypeDelete {
		image = mapped.TargetBefore
	}
	for _, c := range schema.Columns {
		if c.Generated() {
			continue
		}
		if _, ok := image[c.Name]; !ok {
			return fmt.Errorf("conflict_full_target_image_required: %s", c.Name)
		}
	}
	for _, marker := range []string{"last_event_id", "updated_by_node"} {
		if _, ok := image[mapped.TargetColumn(marker)]; !ok {
			return fmt.Errorf("conflict_replay_metadata_required: %s", marker)
		}
	}
	return nil
}
