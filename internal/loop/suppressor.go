package loop

import (
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type ApplyLog interface {
	Exists(eventID string) bool
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

func (s *Suppressor) ShouldUpload(change cdc.ChangeEvent) Decision {
	rule := s.rules.FindForNode(change.DatabaseName, change.TableName, s.localNodeID, s.localNodeID)
	if rule == nil || !rule.Enable {
		return Decision{Upload: false, Reason: "table not in sync rules"}
	}
	if rule.Direction == rules.DirectionIgnore {
		return Decision{Upload: false, Reason: "ignored table"}
	}

	lastEventID := stringValue(change.After, "last_event_id")
	updatedByNode := stringValue(change.After, "updated_by_node")
	if lastEventID != "" && updatedByNode != "" && updatedByNode != s.localNodeID {
		if s.applyLog != nil && s.applyLog.Exists(lastEventID) {
			return Decision{Upload: false, Reason: "replayed sync event"}
		}
	}

	return Decision{Upload: true, Reason: "local business change"}
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
