package cdc

import (
	"bytes"
	"encoding/json"

	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
)

func (change *ChangeEvent) UnmarshalJSON(data []byte) error {
	type wireChange ChangeEvent
	var decoded wireChange
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	if err := rowvalue.Restore(decoded.PrimaryKey, decoded.Before, decoded.After); err != nil {
		return err
	}
	*change = ChangeEvent(decoded)
	return nil
}
