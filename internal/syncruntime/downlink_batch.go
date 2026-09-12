package syncruntime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func (r EdgeDownlinkBatchRuntime) applyDownlinkBatchBodies(ctx context.Context, bodies [][]byte) (int, string, error) {
	entries := make([]mappedBatchEntry, 0, len(bodies))
	committed, lastID := 0, ""
	flush := func() error {
		if len(entries) == 0 {
			return nil
		}
		mapped := make([]mapper.MappedEvent, 0, len(entries))
		for _, entry := range entries {
			if entry.applyable {
				mapped = append(mapped, entry.mapped)
			}
		}
		if len(mapped) > 0 {
			result, err := applyBatch(ctx, r.Worker, mapped)
			if err != nil {
				committed += messageSuccessCountForApplyResults(entries, len(result.Results))
				lastID = eventIDForApplyIndex(entries, len(result.Results))
				return fmt.Errorf("apply downlink batch: %w", err)
			}
		}
		committed += len(entries)
		lastID = entries[len(entries)-1].evt.EventID
		entries = entries[:0]
		return nil
	}
	for _, body := range bodies {
		var raw event.SyncEvent
		if err := json.Unmarshal(cleanJSONBody(body), &raw); err != nil {
			if prior := flush(); prior != nil {
				return committed, lastID, prior
			}
			return committed, "", fmt.Errorf("parse downlink event: %w", err)
		}
		if raw.EventType == event.TypeConfigUpdate {
			// Configuration updates cannot overtake preceding business commits.
			if err := flush(); err != nil {
				return committed, lastID, err
			}
			if r.ConfigStore == nil {
				return committed, raw.EventID, fmt.Errorf("config store is required")
			}
			done := measurePhase(ctx, "config_apply")
			err := r.ConfigStore.UpsertNodeConfig(ctx, nodeConfigFromEvent(raw))
			done()
			if err != nil {
				return committed, raw.EventID, fmt.Errorf("apply downlink config: %w", err)
			}
			committed++
			lastID = raw.EventID
			continue
		}
		evt, mapped, err := mapSyncEvent(body, r.Rules)
		if err != nil {
			if prior := flush(); prior != nil {
				return committed, lastID, prior
			}
			return committed, raw.EventID, err
		}
		rule := findRuleForEvent(r.Rules, evt)
		entry := mappedBatchEntry{evt: evt, mapped: mapped, rule: rule}
		if rule.Enable && rule.Direction != rules.DirectionIgnore && schemaChangeAllowed(evt, mapped, *rule) {
			if err := ensureSyncModeAllowed(mapped, r.AllowCRUDCompact); err != nil {
				if prior := flush(); prior != nil {
					return committed, lastID, prior
				}
				return committed, evt.EventID, err
			}
			entry.applyable = true
			if r.TargetDatabaseOverride != "" {
				entry.mapped.TargetDatabase = rule.DownlinkTargetDatabase(r.TargetDatabaseOverride)
				entry.mapped.Event.DatabaseName = entry.mapped.TargetDatabase
			}
		}
		entries = append(entries, entry)
	}
	if err := flush(); err != nil {
		return committed, lastID, err
	}
	return committed, lastID, nil
}
