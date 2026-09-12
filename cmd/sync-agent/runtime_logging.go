package main

import (
	"io"
	"log/slog"
	"os"

	"github.com/YufeiSun5/NodeBridge/internal/agentlog"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/buildinfo"
)

func runtimeRedactor(cfg *appconfig.Config) func(string) string {
	return agentlog.ForConfig(cfg)
}

func newRuntimeLogger(configPath string, cfg *appconfig.Config, stderr io.Writer, redact func(string) string) (*slog.Logger, io.Closer, error) {
	logger, closer, err := agentlog.New(agentlog.Path(configPath), stderr, redact)
	if err != nil {
		return nil, nil, err
	}
	return logger.With("node_id", cfg.Node.ID, "mode", cfg.Mode, "version", buildinfo.Version, "pid", os.Getpid()), closer, nil
}
