package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
)

func TestUpgradeSystemUnconfiguredAndRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("mode: edge\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runUpgradeSystem([]string{"-config", path, "-allow-unconfigured"}, &output, &output); err != nil || !strings.Contains(output.String(), "not_configured") {
		t.Fatal(err, output.String())
	}
	if err := runUpgradeSystem([]string{"-config", path}, &output, &output); err == nil {
		t.Fatal("unconfigured upgrade accepted without installer flag")
	}
	if err := os.WriteFile(path, []byte("mode: edge\nnode:\n  id: owned-node\nmysql:\n  host: 127.0.0.1\n  port: 1\n  username: owned\n  database: owned\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lock, err := agentstate.Lock(path + ".agent.lock")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := runUpgradeSystem([]string{"-config", path}, &output, &output); err == nil || !strings.Contains(err.Error(), "stop synchronization") {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "system database upgrade failed: stop synchronization") {
		t.Fatal("upgrade failure is missing from CLI diagnostics", output.String())
	}
}
