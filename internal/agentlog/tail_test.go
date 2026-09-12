package agentlog

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTailBoundsFilterAndOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	var content strings.Builder
	content.WriteString(strings.Repeat("x", 1024*1024+1) + "\n")
	for i := 0; i < 600; i++ {
		fmt.Fprintf(&content, "line-%03d\r\n", i)
	}
	if err := os.WriteFile(path, []byte(content.String()), 0600); err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int{0, -1, 501, 2, 500} {
		lines, err := Tail(path, limit)
		want := limit
		if want <= 0 || want > 500 {
			want = 100
		}
		if err != nil || len(lines) != want || lines[0] != fmt.Sprintf("line-%03d", 600-want) || lines[want-1] != "line-599" {
			t.Fatalf("limit %d: lines=%v err=%v", limit, lines, err)
		}
	}
	lines, err := TailFiltered(path, 2, func(line string) bool { return strings.HasSuffix(line, "0") })
	if err != nil || !reflect.DeepEqual(lines, []string{"line-580", "line-590"}) {
		t.Fatalf("filtered=%v err=%v", lines, err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 1024*1024+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if lines, err := Tail(path, 10); err != nil || len(lines) != 0 {
		t.Fatalf("partial=%v err=%v", lines, err)
	}
	if _, err := Tail(path+"missing", 10); !os.IsNotExist(err) {
		t.Fatalf("missing error=%v", err)
	}
}

func TestLogReaderAllowsRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.log")
	if err := os.WriteFile(path, []byte("before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := openLogRead(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatalf("reader blocked rotation: %v", err)
	}
	if err := os.WriteFile(path, []byte("after\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lines, err := Tail(path, 10)
	if err != nil || !reflect.DeepEqual(lines, []string{"after"}) {
		t.Fatalf("tail=%v err=%v", lines, err)
	}
}
