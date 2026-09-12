package main

import (
	"os"
	"path/filepath"
)

func migrationDirectory(executable, scope string) string {
	packaged := filepath.Join(filepath.Dir(executable), "migrations", scope)
	if stat, err := os.Stat(packaged); err == nil && stat.IsDir() {
		return packaged
	}
	return filepath.Join("migrations", scope)
}
