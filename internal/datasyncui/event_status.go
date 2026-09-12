package datasyncui

import (
	"context"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentlog"
	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
	"github.com/YufeiSun5/NodeBridge/internal/eventstatus"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
)

func (a *App) GetEventStatus(req eventstatus.Request) (eventstatus.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.eventStatus(ctx, req)
}

func (a *App) eventStatus(ctx context.Context, req eventstatus.Request) (eventstatus.Response, error) {
	service := eventstatus.Service{LogPath: agentlog.Path(a.effectiveConfigPath()), Redact: agentlog.ForConfig(a.config)}
	if state, err := agentstate.Read(a.effectiveConfigPath()); err == nil && state != nil {
		service.RunningPID = state.PID
	}
	if a.config != nil {
		db, err := mysqlconn.Open(a.config.MySQL)
		if err == nil {
			defer db.Close()
			service.DB = db
		}
	}
	return service.Query(ctx, req)
}
