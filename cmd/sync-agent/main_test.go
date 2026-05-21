package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunEdgeConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"-config", filepath.Join("..", "..", "configs", "edge.example.yaml")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v stderr=%s", err, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{"mode=edge", "node_id=edge-001"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, out)
		}
	}
}

func TestRunServerConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"-config", filepath.Join("..", "..", "configs", "server.example.yaml")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v stderr=%s", err, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{"mode=server", "node_id=server-001"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, out)
		}
	}
}

func TestRunMissingConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"-config", filepath.Join(t.TempDir(), "missing.yaml")}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing config error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "load config failed") {
		t.Fatalf("expected stderr to mention load failure, got %q", stderr.String())
	}
}
