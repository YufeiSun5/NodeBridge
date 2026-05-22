package mcpstdio_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/mcpstdio"
)

func TestServerListsTools(t *testing.T) {
	var stdout bytes.Buffer
	server := mcpstdio.Server{Service: mcpstdio.StaticService{}}

	err := server.Serve(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"), &stdout)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(stdout.String(), "nodebridge_overview") || !strings.Contains(stdout.String(), `"id":1`) {
		t.Fatalf("unexpected tools/list response %s", stdout.String())
	}
}

func TestServerCallsOverviewReadOnly(t *testing.T) {
	var stdout bytes.Buffer
	service := mcpstdio.StaticService{
		ConfigPath: "config.yaml",
		Config: appconfig.Config{
			Mode:     appconfig.ModeServer,
			Node:     appconfig.NodeConfig{ID: "server-001"},
			RabbitMQ: appconfig.RabbitMQConfig{ServerURL: "amqp://sync:secret@127.0.0.1:5672/server-sync"},
		},
	}
	server := mcpstdio.Server{Service: service}

	input := `{"jsonrpc":"2.0","id":"call-1","method":"tools/call","params":{"name":"nodebridge_overview","arguments":{}}}` + "\n"
	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "server-001") || !strings.Contains(out, "mcp_read_only") {
		t.Fatalf("unexpected overview response %s", out)
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("overview response leaked secret: %s", out)
	}
}

func TestServerRejectsUnsupportedTool(t *testing.T) {
	var stdout bytes.Buffer
	server := mcpstdio.Server{Service: mcpstdio.StaticService{}}
	input := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"save_config","arguments":{}}}` + "\n"

	if err := server.Serve(context.Background(), strings.NewReader(input), &stdout); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(stdout.String(), "unsupported tool") {
		t.Fatalf("expected unsupported tool error, got %s", stdout.String())
	}
}
