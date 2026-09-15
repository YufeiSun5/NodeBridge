package mysqlconn

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type systemMigration struct {
	name, checksum string
	statements     []string
}

var systemCreate = regexp.MustCompile(`(?is)^CREATE TABLE IF NOT EXISTS sync_[a-z0-9_]+\s*\(`)

func readSystemMigrations(dir string) ([]systemMigration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var migrations []systemMigration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		migration := systemMigration{name: entry.Name(), checksum: fmt.Sprintf("%x", sha256.Sum256(data))}
		for _, statement := range splitStatements(string(data)) {
			if !systemCreate.MatchString(statement) {
				// The original bootstrap also contains optional demo business tables.
				if entry.Name() == "001_mvp_tables.sql" && strings.HasPrefix(statement, "CREATE TABLE IF NOT EXISTS ") {
					continue
				}
				return nil, fmt.Errorf("unsupported system migration statement: %s", entry.Name())
			}
			migration.statements = append(migration.statements, statement)
		}
		migrations = append(migrations, migration)
	}
	if len(migrations) == 0 {
		return nil, errors.New("system migrations missing")
	}
	return migrations, nil
}

// UpgradeSystem records only completed, retryable system migrations.
func UpgradeSystem(ctx context.Context, db *sql.DB, dir, version string) error {
	migrations, err := readSystemMigrations(dir)
	if err != nil {
		return err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var locked int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(CONCAT('nodebridge-upgrade:', DATABASE()), 10)").Scan(&locked); err != nil {
		return err
	}
	if locked != 1 {
		return errors.New("system database upgrade busy")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(cleanup, "DO RELEASE_LOCK(CONCAT('nodebridge-upgrade:', DATABASE()))")
	}()
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS sync_schema_migration (migration_name VARCHAR(128) CHARACTER SET ascii PRIMARY KEY, checksum CHAR(64) CHARACTER SET ascii NOT NULL, applied_version VARCHAR(32) NOT NULL, applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB`); err != nil {
		return err
	}
	for _, migration := range migrations {
		var checksum string
		err := conn.QueryRowContext(ctx, "SELECT checksum FROM sync_schema_migration WHERE migration_name=?", migration.name).Scan(&checksum)
		if err == nil {
			if checksum != migration.checksum {
				return fmt.Errorf("system migration checksum changed: %s", migration.name)
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		for _, statement := range migration.statements {
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("system migration %s: %w", migration.name, err)
			}
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO sync_schema_migration (migration_name,checksum,applied_version) VALUES (?,?,?)", migration.name, migration.checksum, version); err != nil {
			return err
		}
	}
	return nil
}
