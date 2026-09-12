package agentlog

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
)

func Path(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "logs", "sync-runtime.jsonl")
}

// Tail bounds IO and memory even if a legacy log has never been rotated.
func Tail(path string, limit int) ([]string, error) {
	return TailFiltered(path, limit, nil)
}

func TailFiltered(path string, limit int, include func(string) bool) ([]string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	f, err := openLogRead(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	const maxTail = 1024 * 1024
	start := max(int64(0), info.Size()-maxTail)
	data, err := io.ReadAll(io.NewSectionReader(f, start, info.Size()-start))
	if err != nil {
		return nil, err
	}
	if start > 0 {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			data = data[i+1:]
		} else {
			return nil, nil
		}
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	result := make([]string, 0, limit)
	for i := len(lines) - 1; i >= 0 && len(result) < limit; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" && (include == nil || include(line)) {
			result = append(result, line)
		}
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}
