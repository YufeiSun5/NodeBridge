package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentlog"
	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/buildinfo"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
)

func runUpgradeSystem(args []string, stdout, stderr io.Writer) (runErr error) {
	redact := agentlog.ForConfig(nil)
	defer func() {
		if runErr != nil {
			fmt.Fprintln(stderr, redact("system database upgrade failed: "+runErr.Error()))
		}
	}()
	flags := flag.NewFlagSet("upgrade-system", flag.ContinueOnError)
	flags.SetOutput(stderr)
	config := flags.String("config", appconfig.DefaultConfigPath(), "existing node configuration")
	allowUnconfigured := flags.Bool("allow-unconfigured", false, "allow a fresh unconfigured installation")
	createDatabase := flags.Bool("create-database", false, "explicitly create the configured system database before upgrading system tables")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := appconfig.LoadFileAllowIncomplete(*config)
	if err != nil {
		return err
	}
	redact = agentlog.ForConfig(cfg)
	if cfg.Node.ID == "" && cfg.MySQL.Database == "" && *allowUnconfigured {
		return json.NewEncoder(stdout).Encode(map[string]string{"status": "not_configured", "version": buildinfo.Version})
	}
	if (cfg.Mode != "edge" && cfg.Mode != "server") || cfg.Node.ID == "" || cfg.MySQL.Host == "" || cfg.MySQL.Port <= 0 || cfg.MySQL.Username == "" || cfg.MySQL.Database == "" {
		return errors.New("system upgrade requires configured node role and MySQL connection")
	}
	lock, err := agentstate.Lock(*config + ".agent.lock")
	if err != nil {
		return fmt.Errorf("stop synchronization before system upgrade: %w", err)
	}
	defer lock.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if *createDatabase {
		if err := mysqlconn.CreateSystemDatabase(ctx, cfg.MySQL); err != nil {
			return err
		}
	}
	db, err := mysqlconn.Open(cfg.MySQL)
	if err != nil {
		return err
	}
	defer db.Close()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := mysqlconn.UpgradeSystem(ctx, db, migrationDirectory(exe, cfg.Mode), buildinfo.Version); err != nil {
		return err
	}
	if err := mysqlconn.EnsureApplyDiagnosticIndexes(ctx, db); err != nil {
		return err
	}
	if cfg.Mode == "server" {
		if err := mysqlconn.EnsureServerDiagnosticIndexes(ctx, db); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS sync_schema_version (scope VARCHAR(16) CHARACTER SET ascii PRIMARY KEY, version VARCHAR(32) NOT NULL, upgraded_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO sync_schema_version (scope,version) VALUES (?,?) ON DUPLICATE KEY UPDATE version=VALUES(version),upgraded_at=CURRENT_TIMESTAMP", cfg.Mode, buildinfo.Version); err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(map[string]string{"status": "upgraded", "scope": cfg.Mode, "database": cfg.MySQL.Database, "version": buildinfo.Version})
}
