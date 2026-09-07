//go:build !windows

package main

import (
	"fmt"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/mcpstdio"
)

func newMCPService(config, rules, log string, lab bool) (mcpstdio.Service, error) {
	if lab {
		return nil, fmt.Errorf("lab management server currently requires Windows; connect to Windows using SSH")
	}
	cfg, err := appconfig.LoadFile(config)
	if err != nil {
		return nil, err
	}
	if !cfg.MCP.Enable {
		return nil, fmt.Errorf("mcp_server.enable must be true to start mcp-stdio")
	}
	return mcpstdio.StaticService{ConfigPath: config, RulesPath: rules, LogPath: log, Config: *cfg}, nil
}
