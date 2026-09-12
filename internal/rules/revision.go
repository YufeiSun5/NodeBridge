package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
)

const MissingRevision = "missing"

func revision(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func FileRevision(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return MissingRevision, nil
	}
	if err != nil {
		return "", err
	}
	return revision(data), nil
}

func LoadFileWithRevision(path string) (*RuleSet, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read rules %q: %w", path, err)
	}
	set, err := parseFile(data, path)
	return set, revision(data), err
}

func lockRules(path string) (*os.File, error) {
	deadline := time.Now().Add(5 * time.Second)
	for {
		file, err := agentstate.Lock(path + ".write.lock")
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, agentstate.ErrRunning) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("rules_busy: another writer holds the rules lock")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func SaveFileCAS(path string, set RuleSet, expected string) (string, error) {
	if err := set.Validate(); err != nil {
		return "", err
	}
	lock, err := lockRules(path)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	current, err := FileRevision(path)
	if err != nil {
		return "", err
	}
	if expected == "" && current != MissingRevision {
		return "", fmt.Errorf("expected_revision_required: read rules before replacing the existing array")
	}
	if expected != "" && expected != current {
		return "", fmt.Errorf("revision_conflict: rules changed since they were read; reload before saving")
	}
	if err := saveFileUnlocked(path, set); err != nil {
		return "", err
	}
	return FileRevision(path)
}
