package main

import (
	"github.com/YufeiSun5/NodeBridge/internal/datasyncui"
	"github.com/YufeiSun5/NodeBridge/internal/mcpstdio"
)

func newMCPService(config, rules, log string, lab bool) (mcpstdio.Service, error) {
	return datasyncui.NewMCPService(config, rules, log, lab)
}
