package apply

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/conflict"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type RepairWorker struct {
	DB           *sql.DB
	NodeID       string
	Rules        *rules.RuleSet
	CaptureFence CaptureFence
}

// RunOnce repairs one durable candidate without allocating a new business version.
func (w RepairWorker) RunOnce(ctx context.Context) (bool, error) {
	if w.DB == nil || w.NodeID == "" || w.Rules == nil || w.CaptureFence == nil {
		return false, errors.New("conflict_repair_dependencies_required")
	}
	for _, candidate := range w.Rules.Rules {
		rule := w.Rules.FindForNode(candidate.DatabaseName, candidate.TableName, w.NodeID, w.NodeID)
		if rule == nil || !rule.Enable || rule.Direction != rules.DirectionBidirectional || rule.ConflictPolicy != rules.ConflictLastWriteWin {
			continue
		}
		key, state, found, err := conflict.NextTableRepair(ctx, w.DB, rule.DatabaseName, rule.TableName)
		if err != nil {
			return false, err
		}
		if found {
			return w.repair(ctx, key, state)
		}
	}
	return false, nil
}

func (w RepairWorker) repair(ctx context.Context, key conflict.RowKey, candidate conflict.RowState) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	tx, err := w.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	mapped := repairMapped(key, candidate, w.NodeID, "pending-repair")
	if err := validateMappedEvent(mapped); err != nil {
		return false, err
	}
	if err := checkTargetSchemas(ctx, tx, []mapper.MappedEvent{mapped}); err != nil {
		return false, err
	}
	schema, err := rulecheck.ReadSchema(ctx, tx, key.Database, key.Table)
	if err != nil {
		return false, err
	}
	actualKey, err := conflict.CanonicalKey(ctx, tx, schema, candidate.PrimaryKey)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(actualKey.CanonicalKey, key.CanonicalKey) {
		return false, errors.New("conflict_repair_key_changed")
	}
	exists, err := lockConflictRow(ctx, tx, mapped)
	if err != nil {
		return false, err
	}
	if err := w.CaptureFence.Wait(ctx); err != nil {
		return false, err
	}
	if err := conflict.PinKeySchema(ctx, tx, schema); err != nil {
		return false, err
	}
	state, required, err := conflict.ReadWinnerState(ctx, tx, key)
	if err != nil {
		return false, err
	}
	if !required {
		return false, nil
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return false, err
	}
	mapped = repairMapped(key, state, w.NodeID, "repair-"+hex.EncodeToString(random[:]))
	if state.Version.Deleted && state.DeleteMode == rules.DeleteHard {
		// A tombstone has no row image, but still needs schema and marker checks.
		mapped.TargetBefore = make(map[string]any, len(schema.Columns))
		for _, column := range schema.Columns {
			mapped.TargetBefore[column.Name] = nil
		}
	} else if exists {
		mapped.Event.EventType = event.TypeUpdate
	} else {
		mapped.Event.EventType = event.TypeInsert
	}
	if err := validateConflictTarget(ctx, tx, schema, mapped); err != nil {
		return false, err
	}
	physicalWrite := true
	switch mapped.Event.EventType {
	case event.TypeDelete:
		if exists {
			err = applyHardDelete(ctx, tx, mapped)
		} else {
			physicalWrite = false
		}
	case event.TypeInsert:
		err = applyInsert(ctx, tx, mapped)
	case event.TypeUpdate:
		err = applyUpdate(ctx, tx, mapped)
	default:
		err = errors.New("conflict_repair_operation_invalid")
	}
	if err != nil {
		return false, err
	}
	if physicalWrite {
		if _, err := tx.ExecContext(ctx, "INSERT INTO sync_repair_replay (event_id,node_id,database_name,table_name,operation) VALUES (?,?,?,?,?)", mapped.Event.EventID, w.NodeID, key.Database, key.Table, mapped.Event.EventType); err != nil {
			return false, err
		}
	}
	if err := insertApplyLog(ctx, tx, mapped, time.Now()); err != nil {
		return false, err
	}
	if err := conflict.SetRepairRequired(ctx, tx, key, false); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit conflict repair (outcome may be unknown): %w", err)
	}
	return true, nil
}

func repairMapped(key conflict.RowKey, state conflict.RowState, node, id string) mapper.MappedEvent {
	operation := event.TypeUpdate
	if state.Version.Deleted && state.DeleteMode == rules.DeleteHard {
		operation = event.TypeDelete
	}
	return mapper.MappedEvent{Event: event.SyncEvent{EventID: id, OriginNodeID: node, SourceNodeID: node, EventType: operation, EventTime: state.Version.Time}, SourceDatabase: key.Database, SourceTable: key.Table, TargetDatabase: key.Database, TargetTable: key.Table, TargetPrimaryKey: state.PrimaryKey, TargetBefore: state.Row, TargetAfter: state.Row, TargetColumns: state.Columns, DeleteMode: rules.DeleteHard, TrackDeleteReplay: true}
}
