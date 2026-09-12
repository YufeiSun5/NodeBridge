package conflict

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/event"
)

// FromEvent must run before table mapping. Relay-local fields never enter the hash.
func FromEvent(evt event.SyncEvent) (Version, error) {
	if evt.EventType != event.TypeInsert && evt.EventType != event.TypeUpdate && evt.EventType != event.TypeDelete {
		return Version{}, errors.New("conflict_unsupported_event_type")
	}
	if evt.DatabaseName == "" || evt.TableName == "" || len(evt.PrimaryKey) == 0 {
		return Version{}, errors.New("conflict_row_identity_required")
	}
	for _, v := range evt.PrimaryKey {
		if v == nil {
			return Version{}, errors.New("conflict_null_primary_key")
		}
	}
	v := Version{Time: evt.EventTime.UTC(), TimeSource: evt.Headers["event_time_source"], OriginNodeID: evt.OriginNodeID, EventID: evt.EventID, Deleted: evt.EventType == event.TypeDelete}
	v.BinlogFile, v.BinlogPos = evt.BinlogFile, evt.BinlogPos
	if v.TimeSource == SourceBinlog && (evt.BinlogFile == "" || evt.BinlogPos == 0) {
		return Version{}, errors.New("conflict_source_position_required")
	}
	payload := struct {
		Type     string         `json:"type"`
		Database string         `json:"database"`
		Table    string         `json:"table"`
		Key      map[string]any `json:"key"`
		Before   map[string]any `json:"before"`
		After    map[string]any `json:"after"`
	}{evt.EventType, evt.DatabaseName, evt.TableName, evt.PrimaryKey, evt.Before, evt.After}
	b, err := json.Marshal(payload)
	if err != nil {
		return Version{}, fmt.Errorf("conflict_payload_invalid: %w", err)
	}
	sum := sha256.Sum256(b)
	v.PayloadHash = hex.EncodeToString(sum[:])
	return v, v.Validate()
}
