package mcpstdio

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestProtocolEnvelope(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"parse", `{`, `"code":-32700`},
		{"version", `{"id":1,"method":"ping"}`, `"code":-32600`},
		{"invalid id", `{"jsonrpc":"2.0","id":true,"method":"ping"}`, `"code":-32600`},
		{"large id", `{"jsonrpc":"2.0","id":9007199254740993,"method":"ping"}`, `"id":9007199254740993`},
		{"zero id", `{"jsonrpc":"2.0","id":0,"method":"ping"}`, `"id":0`},
		{"method", `{"jsonrpc":"2.0","id":1,"method":"missing"}`, `"code":-32601`},
		{"tool", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"missing"}}`, `"code":-32602`},
		{"arguments", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nodebridge_logs","arguments":[]}}`, `"code":-32602`},
		{"tool failure", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nodebridge_save_config_patch","arguments":{}}}`, `"isError":true`},
		{"negotiation", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`, `"protocolVersion":"2025-06-18"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := (Server{Service: StaticService{}}).Serve(context.Background(), strings.NewReader(tc.input+"\n"), &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("got %s, want %s", out.String(), tc.want)
			}
			if tc.name == "parse" && !strings.Contains(out.String(), `"id":null`) {
				t.Fatal("missing null id")
			}
		})
	}
	var out bytes.Buffer
	_ = (Server{Service: StaticService{}}).Serve(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","method":"ping"}`+"\n"), &out)
	if out.Len() != 0 {
		t.Fatal("responded to notification")
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }
func TestOutputFailure(t *testing.T) {
	err := (Server{Service: StaticService{}}).Serve(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n"), failedWriter{})
	if err == nil {
		t.Fatal("ignored broken pipe")
	}
}

func TestLogLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.log")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines := readLogTail(path, 1)
	if len(lines) != 1 || lines[0] != "three" {
		t.Fatalf("unexpected tail: %v", lines)
	}
}

func TestSSHCommandPreservesPaths(t *testing.T) {
	exe := `C:\Program Files\NodeBridge\app\SyncAgent.exe`
	path := `C:\lab's data\config.yaml`
	args, err := SSHArguments("lab@10.0.0.2", 2222, "/Users/lab/.ssh/id_ed25519", exe, []string{"mcp-stdio", "-config", path, "-lab-full-access"})
	if err != nil {
		t.Fatal(err)
	}
	last := args[len(args)-1]
	data, err := base64.StdEncoding.DecodeString(last[strings.LastIndex(last, " ")+1:])
	if err != nil {
		t.Fatal(err)
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	script := string(utf16.Decode(units))
	if !strings.Contains(script, "'"+exe+"'") || !strings.Contains(script, "lab''s data") || !strings.Contains(script, "-lab-full-access") {
		t.Fatal(script)
	}
	encoded, _ := json.Marshal(args)
	if !strings.Contains(string(encoded), "BatchMode=yes") || !strings.Contains(string(encoded), "2222") {
		t.Fatal(string(encoded))
	}
	if _, err := SSHArguments("-oProxyCommand=bad", 22, "", exe, nil); err == nil {
		t.Fatal("accepted option as host")
	}
}
