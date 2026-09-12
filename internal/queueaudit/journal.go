package queueaudit

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/YufeiSun5/NodeBridge/internal/atomicfile"
)

// FileJournal is protected by the caller's cross-process operation lock.
type FileJournal struct{ Directory string }

func (j FileJournal) path(id string) (string, error) {
	if len(id) != 64 {
		return "", errors.New("invalid queue plan ID")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return "", errors.New("invalid queue plan ID")
	}
	return filepath.Join(j.Directory, id+".json"), nil
}

func (j FileJournal) Load(id string) (Receipt, error) {
	path, err := j.path(id)
	if err != nil {
		return Receipt{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return Receipt{}, err
	}
	sealed, err := Seal(receipt.Plan)
	if err != nil || sealed.ID != id || receipt.Plan.ID != id {
		return Receipt{}, errors.New("queue journal integrity failure")
	}
	return receipt, nil
}

func (j FileJournal) Save(receipt Receipt) error {
	path, err := j.path(receipt.Plan.ID)
	if err != nil {
		return err
	}
	sealed, err := Seal(receipt.Plan)
	if err != nil || sealed.ID != receipt.Plan.ID {
		return errors.New("queue journal integrity failure")
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	return atomicfile.Write(path, data, 0o600)
}
