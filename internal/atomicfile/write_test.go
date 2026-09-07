package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "config.yaml")
	for _, want := range []string{"initial", "new contents"} {
		if err := Write(path, []byte(want), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("replace: %q %v", got, err)
		}
	}
	items, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(items) != 1 {
		t.Fatal("temporary files left behind")
	}
}
