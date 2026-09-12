package datasyncui

import (
	"context"

	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/mysqldiag"
)

func (s *MCPService) mysqlDiagnostics(ctx context.Context, req mysqldiag.Request) (mysqldiag.Report, error) {
	if err := req.Validate(); err != nil {
		return mysqldiag.Report{}, err
	}
	db, err := mysqlconn.Open(s.Config.MySQL)
	if err != nil {
		return mysqldiag.Report{}, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	return mysqldiag.Collect(ctx, db, s.Config.MySQL.Database, req)
}
