package agentlog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRedactor(t *testing.T) {
	redact := Redactor("p@ss/word", "quoted\"secret", "red", "redacted-secret")
	for _, text := range []string{
		"connect mysql p@ss/word and p%40ss%2Fword",
		"amqp://unknown:other-password@host/vhost",
		"unknown:another-password@tcp(host:3306)/db",
		`duplicate entry 'customer-email@example.com' for key 'PRIMARY'`,
		`syntax near 'value\'s and private' at line 1`,
		`password=arbitrary token=top-secret authorization=abc`,
		`quoted\"secret redacted-secret`,
	} {
		got := redact(text)
		for _, secret := range []string{"p@ss/word", "p%40ss%2Fword", "other-password", "another-password", "customer-email", "private", "arbitrary", "top-secret", "abc", "quoted\\\"secret", "redacted-secret"} {
			if strings.Contains(got, secret) {
				t.Fatalf("secret %q retained: %s", secret, got)
			}
		}
	}
	if got := Redactor()("apply update: Error 1205 (HY000): Lock wait timeout exceeded"); !strings.Contains(got, "1205 (HY000)") {
		t.Fatalf("lost diagnostic code: %s", got)
	}
	if len(Redactor()(strings.Repeat("x", 10000))) > 8220 {
		t.Fatal("message not bounded")
	}
}

func TestStructuredLogIsPersistentAndRedacted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "runtime.jsonl")
	var fallback bytes.Buffer
	logger, closer, err := New(path, &fallback, Redactor("configured-secret"))
	if err != nil {
		t.Fatal(err)
	}
	logger.With("node_id", "edge-1").Error("worker failed", "worker", "edge-downlink", "event_id", "evt-1", "error", errors.New("apply update: configured-secret; duplicate 'private-value'"), "phase_ms", map[string]float64{"mysql_apply": 12.5})
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("configured-secret")) || bytes.Contains(data, []byte("private-value")) {
		t.Fatal("secret persisted")
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record["event_id"] != "evt-1" || record["node_id"] != "edge-1" || record["level"] != "ERROR" || record["phase_ms"] == nil || record["time"] == nil {
		t.Fatalf("missing context: %s", data)
	}
	if fallback.Len() != 0 {
		t.Fatalf("unexpected fallback: %s", fallback.String())
	}
}

func TestRotationBoundsAndPreservesWholeRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.jsonl")
	w, err := openRotating(path, 40, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if _, err := fmt.Fprintf(w, "{\"n\":%d}\n", i); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(path + "*")
	if err != nil || len(files) != 3 {
		t.Fatalf("files=%v err=%v", files, err)
	}
	var numbers []int
	for _, file := range []string{path + ".2", path + ".1", path} {
		data, err := os.ReadFile(file)
		if err != nil || len(data) > 40 {
			t.Fatalf("invalid size: %d %v", len(data), err)
		}
		for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
			var value struct {
				N int `json:"n"`
			}
			if err := json.Unmarshal(line, &value); err != nil {
				t.Fatal(err)
			}
			numbers = append(numbers, value.N)
		}
	}
	for i := 1; i < len(numbers); i++ {
		if numbers[i] != numbers[i-1]+1 {
			t.Fatal("rotation changed order")
		}
	}
	if numbers[len(numbers)-1] != 19 {
		t.Fatal("newest record missing")
	}
	if _, err := w.Write([]byte("closed")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed write: %v", err)
	}
}

func TestRotationFailureUsesFallbackAndRecovers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.jsonl")
	var fallback bytes.Buffer
	w, err := openRotating(path, 8, 1, &fallback)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err := w.Write([]byte("first\n")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".1", 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("second\n")); err == nil {
		t.Fatal("rotation error hidden")
	}
	if !strings.Contains(fallback.String(), "unavailable") || !strings.Contains(fallback.String(), "second") {
		t.Fatal("fallback missing")
	}
	if err := os.Remove(path + ".1"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("third\n")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fallback.String(), "recovered") {
		t.Fatal("recovery not visible")
	}
	old, err := os.ReadFile(path + ".1")
	if err != nil || string(old) != "first\n" {
		t.Fatalf("original log damaged: %q %v", old, err)
	}
}

func TestConcurrentLogRecordsAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.jsonl")
	logger, closer, err := New(path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				logger.Error("failure", "error", errors.New("db offline"))
			}
		}()
	}
	wg.Wait()
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	logger, closer, err = New(path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("restarted")
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) != 201 {
		t.Fatalf("records=%d", len(lines))
	}
	for _, line := range lines {
		if !json.Valid(line) {
			t.Fatalf("torn record %q", line)
		}
	}
}

func TestOpenLogFailureAndOversize(t *testing.T) {
	path := t.TempDir()
	if _, _, err := New(path, nil, nil); err == nil {
		t.Fatal("directory accepted as logfile")
	}
	w, err := openRotating(filepath.Join(path, "small"), 8, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err := w.Write([]byte("too long record")); err == nil {
		t.Fatal("oversize accepted")
	}
	if _, err := w.Write([]byte("ok\n")); err != nil {
		t.Fatal(err)
	}
}
