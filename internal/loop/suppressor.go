package loop

import (
	"context"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type ApplyLog interface {
	Exists(ctx context.Context, eventID string) (bool, error)
}

type DeleteReplayLog interface {
	IsDeleteReplay(ctx context.Context, eventID, originNodeID, database, table string) (bool, error)
}

type RepairReplayLog interface {
	IsRepairReplay(context.Context, string, string, string, string, string) (bool, error)
}

type Suppressor struct {
	localNodeID string
	rules       rules.RuleSet
	applyLog    ApplyLog
}

type Decision struct {
	Upload bool
	Reason string
}

func NewSuppressor(localNodeID string, ruleSet rules.RuleSet, applyLog ApplyLog) *Suppressor {
	return &Suppressor{
		localNodeID: localNodeID,
		rules:       ruleSet,
		applyLog:    applyLog,
	}
}

func (s *Suppressor) ShouldUpload(ctx context.Context, change cdc.ChangeEvent) (Decision, error) {
	if err := ctx.Err(); err != nil {
		return Decision{}, err
	}
	rule := s.rules.FindForNode(change.DatabaseName, change.TableName, s.localNodeID, s.localNodeID)
	if rule == nil || !rule.Enable {
		return Decision{Upload: false, Reason: "table not in sync rules"}, nil
	}
	if rule.Direction == rules.DirectionIgnore {
		return Decision{Upload: false, Reason: "ignored table"}, nil
	}
	if change.SchemaChange != nil {
		switch change.Operation {
		case cdc.OperationAddColumn:
			if !rule.SchemaSync.AddColumns {
				return Decision{Upload: false, Reason: "ADD COLUMN synchronization is disabled"}, nil
			}
		case cdc.OperationDropColumn:
			if !rule.SchemaSync.DropColumns {
				return Decision{Upload: false, Reason: "DROP COLUMN synchronization is disabled"}, nil
			}
			if rule.Direction != rules.DirectionServerToEdge && len(rule.SourceNodeIDs) != 1 {
				return Decision{Upload: false, Reason: "DROP COLUMN requires exactly one source node"}, nil
			}
		}
	}

	eventColumn := sourceMetadataColumn(*rule, "last_event_id")
	nodeColumn := sourceMetadataColumn(*rule, "updated_by_node")
	if change.Operation == cdc.OperationDelete {
		last, by := stringValue(change.Before, eventColumn), stringValue(change.Before, nodeColumn)
		if last != "" && by == s.localNodeID {
			if log, ok := s.applyLog.(RepairReplayLog); ok {
				replay, err := log.IsRepairReplay(ctx, last, by, change.DatabaseName, change.TableName, "DELETE")
				if err != nil {
					return Decision{}, err
				}
				if replay {
					return Decision{Upload: false, Reason: "local conflict repair delete"}, nil
				}
			}
		}
		if last != "" && by != "" && by != s.localNodeID {
			log, ok := s.applyLog.(DeleteReplayLog)
			if !ok {
				return Decision{}, fmt.Errorf("delete replay log is required for %s.%s", change.DatabaseName, change.TableName)
			}
			replay, err := log.IsDeleteReplay(ctx, last, by, change.DatabaseName, change.TableName)
			if err != nil {
				return Decision{}, fmt.Errorf("check delete replay: %w", err)
			}
			if replay {
				return Decision{Upload: false, Reason: "replayed sync delete"}, nil
			}
		}
		return Decision{Upload: true, Reason: "local business delete"}, nil
	}
	lastEventID := stringValue(change.After, eventColumn)
	updatedByNode := stringValue(change.After, nodeColumn)
	if lastEventID != "" && updatedByNode == s.localNodeID {
		retained := change.Operation == cdc.OperationUpdate && stringValue(change.Before, eventColumn) == lastEventID
		if !retained {
			if log, ok := s.applyLog.(RepairReplayLog); ok {
				replay, err := log.IsRepairReplay(ctx, lastEventID, updatedByNode, change.DatabaseName, change.TableName, string(change.Operation))
				if err != nil {
					return Decision{}, err
				}
				if replay {
					if change.Operation == cdc.OperationUpdate {
						if _, ok := change.Before[eventColumn]; !ok {
							return Decision{}, fmt.Errorf("replay_before_image_required: repair needs FULL row images")
						}
					}
					return Decision{Upload: false, Reason: "local conflict repair"}, nil
				}
			}
		}
	}
	if lastEventID != "" && updatedByNode != "" && updatedByNode != s.localNodeID {
		if change.Operation == cdc.OperationUpdate {
			_, hasEvent := change.Before[eventColumn]
			_, hasNode := change.Before[nodeColumn]
			if !hasEvent || !hasNode {
				return Decision{}, fmt.Errorf("replay_before_image_required: %s.%s requires FULL binlog row images", change.DatabaseName, change.TableName)
			}
			// Later local writes can retain metadata from the last remote apply.
			if stringValue(change.Before, eventColumn) == lastEventID && stringValue(change.Before, nodeColumn) == updatedByNode {
				return Decision{Upload: true, Reason: "local business change with retained replay markers"}, nil
			}
		}
		if s.applyLog == nil {
			return Decision{}, fmt.Errorf("apply log is required to check replay event %s", lastEventID)
		}
		exists, err := s.applyLog.Exists(ctx, lastEventID)
		if err != nil {
			return Decision{}, fmt.Errorf("check replay event %s on %s.%s: %w", lastEventID, change.DatabaseName, change.TableName, err)
		}
		if exists {
			return Decision{Upload: false, Reason: "replayed sync event"}, nil
		}
	}

	return Decision{Upload: true, Reason: "local business change"}, nil
}

func sourceMetadataColumn(rule rules.SyncRule, name string) string {
	for _, mapping := range rule.ColumnMappings {
		if mapping.TargetColumn == name {
			return mapping.SourceColumn
		}
	}
	return name
}

func stringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
