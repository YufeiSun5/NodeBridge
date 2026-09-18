package mysqlconn

import (
	"context"
	"fmt"
	"regexp"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

var databaseIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_$-]{0,63}$`)

// CreateSystemDatabase is explicit provisioning, never a connection-test side effect.
func CreateSystemDatabase(ctx context.Context, cfg appconfig.MySQLConfig) error {
	if !databaseIdentifier.MatchString(cfg.Database) {
		return fmt.Errorf("invalid system database identifier")
	}
	name := cfg.Database
	cfg.Database = ""
	db, err := Open(cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS `"+name+"` CHARACTER SET utf8mb4"); err != nil {
		return fmt.Errorf("initialize system database (requires CREATE permission): %w", err)
	}
	return nil
}
