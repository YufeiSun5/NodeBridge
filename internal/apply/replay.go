package apply

import "github.com/YufeiSun5/NodeBridge/internal/mapper"

func replayWrite(mapped mapper.MappedEvent) mapper.MappedEvent {
	last, by := mapped.TargetColumn("last_event_id"), mapped.TargetColumn("updated_by_node")
	_, hasLast := mapped.TargetAfter[last]
	_, hasBy := mapped.TargetAfter[by]
	if !hasLast || !hasBy || mapped.Event.OriginNodeID == "" {
		return mapped
	}
	// Never mutate the source event forwarded to another node or retained for replay.
	after := make(map[string]any, len(mapped.TargetAfter))
	for name, value := range mapped.TargetAfter {
		after[name] = value
	}
	after[last], after[by] = mapped.Event.EventID, mapped.Event.OriginNodeID
	mapped.TargetAfter = after
	return mapped
}
