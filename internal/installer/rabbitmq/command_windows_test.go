package rabbitmq

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBatchArgumentHelper(t *testing.T) {
	if os.Getenv("NB_BATCH_PROBE") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			_ = json.NewEncoder(os.Stdout).Encode(os.Args[i+1:])
			os.Exit(0)
		}
	}
	os.Exit(2)
}

func TestWindowsBatchPreservesPermissionArguments(t *testing.T) {
	t.Setenv("NB_BATCH_PROBE", "1")
	dir := filepath.Join(t.TempDir(), "RabbitMQ Server (test)")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rabbitmqctl.bat")
	// Model the batch-to-executable forwarding performed by rabbitmqctl.bat.
	if err := os.WriteFile(path, []byte("@echo off\r\n\""+exe+"\" -test.run=^TestBatchArgumentHelper$ -- %*\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"set_permissions", "-p", "/nodebridge-server", "nb-edge-001", `^nb\.alignment\.(session\.[0-9a-f]{32}|offer\.[0-9a-f]{64}|[0-9a-f]{64}\.(frames|replies))$`, `^(server\.ingress\..*|amq\.default)$`, `^(edge-001\.downlink\..*|nb\.alignment\..*)$`, "", "space value", "a&b<c>d^e"}
	out, err := runCommand(context.Background(), path, args...)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%s: %v", out, err)
	}
	if !reflect.DeepEqual(got, args) {
		t.Fatalf("got %#v, want %#v", got, args)
	}
	for _, bad := range []string{"%PATH%", "!PATH!", "\" & echo wrong", "a\nb", "a\rb", "a\x00b"} {
		if _, err := runCommand(context.Background(), path, bad); err == nil || !strings.Contains(err.Error(), "unsupported character") {
			t.Fatalf("unsafe argument accepted: %q: %v", bad, err)
		}
	}
}

func TestWindowsBatchReturnsExitCode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "failure.cmd")
	if err := os.WriteFile(path, []byte("@echo off\r\necho controlled-failure\r\nexit /b 23\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := runCommand(context.Background(), path)
	if err == nil || !strings.Contains(err.Error(), "exit status 23") || !strings.Contains(out, "controlled-failure") {
		t.Fatalf("%q %v", out, err)
	}
}
