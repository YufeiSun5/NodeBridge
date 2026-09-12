package event

import (
	"bytes"
	"encoding/json"

	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
)

// Keep numeric row values exact at every consumer, replay and diagnostic boundary.
func (e *SyncEvent) UnmarshalJSON(data []byte) error {
	type wireEvent SyncEvent
	var decoded wireEvent
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	if err := rowvalue.Restore(decoded.PrimaryKey, decoded.Before, decoded.After); err != nil {
		return err
	}
	*e = SyncEvent(decoded)
	return nil
}
