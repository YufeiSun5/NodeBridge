package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedCanalSnapshotBufferFloor(t *testing.T) {
	for _, tc := range []struct {
		name, before, size, unit string
		invalid                  bool
	}{
		{"default", "", "16384", "32768", false},
		{"old", "canal.instance.memory.buffer.size = 16384\ncanal.instance.memory.buffer.memunit = 1024\n", "16384", "32768", false},
		{"larger", "canal.instance.memory.buffer.size = 32768\ncanal.instance.memory.buffer.memunit = 65536\n", "32768", "65536", false},
		{"invalid", "canal.instance.memory.buffer.memunit = invalid\n", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "conf")
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "canal.properties")
			if err := os.WriteFile(path, []byte(tc.before), 0o600); err != nil {
				t.Fatal(err)
			}
			err := activateCanalDestination(root, "owned")
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid capacity silently replaced")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			first, _ := os.ReadFile(path)
			for _, want := range []string{"canal.instance.memory.buffer.size = " + tc.size, "canal.instance.memory.buffer.memunit = " + tc.unit} {
				if !strings.Contains(string(first), want) {
					t.Fatal("capacity floor or custom value lost", string(first))
				}
			}
			if err := activateCanalDestination(root, "owned"); err != nil {
				t.Fatal(err)
			}
			second, _ := os.ReadFile(path)
			if string(first) != string(second) {
				t.Fatal("capacity repair is not idempotent")
			}
		})
	}
}
