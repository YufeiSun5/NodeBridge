package main

import (
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/status"
)

// App is the Wails-facing backend shell. It intentionally avoids Wails-specific
// imports until the frontend scaffold is generated.
type App struct {
	config *appconfig.Config
}

func NewApp() *App {
	return &App{}
}

func (a *App) GetOverview() status.Overview {
	return status.Overview{
		ProductName: "DataSync",
		Mode:        appconfig.ModeUnknown,
		AgentStatus: status.AgentStopped,
		Version:     "0.1.0",
	}
}

func (a *App) LoadConfig(path string) (*appconfig.Config, error) {
	cfg, err := appconfig.LoadFile(path)
	if err != nil {
		return nil, err
	}
	a.config = cfg
	return cfg, nil
}
