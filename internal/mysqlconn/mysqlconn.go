package mysqlconn

import (
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/go-sql-driver/mysql"
)

func DSN(cfg appconfig.MySQLConfig) string {
	mysqlCfg := mysql.NewConfig()
	mysqlCfg.User = cfg.Username
	mysqlCfg.Passwd = cfg.Password
	mysqlCfg.Net = "tcp"
	mysqlCfg.Addr = net.JoinHostPort(cfg.Host, fmt.Sprint(cfg.Port))
	mysqlCfg.DBName = cfg.Database
	mysqlCfg.ParseTime = true
	mysqlCfg.Loc = time.Local
	mysqlCfg.Params = map[string]string{"charset": "utf8mb4"}
	return mysqlCfg.FormatDSN()
}

func Open(cfg appconfig.MySQLConfig) (*sql.DB, error) {
	return OpenDSN(DSN(cfg))
}

func OpenDSN(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	return db, nil
}
