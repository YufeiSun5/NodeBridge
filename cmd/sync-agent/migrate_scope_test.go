package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateRejectsUnknownScopeBeforeConnecting(t *testing.T) {
	var out, stderr bytes.Buffer
	for _, scope := range []string{"other", "../server", "server/../edge"} {
		err := runMigrate([]string{"-scope", scope, "-config", "missing-config.yaml"}, &out, &stderr)
		if err == nil || !strings.Contains(err.Error(), "migration scope must be edge or server") {
			t.Fatalf("scope=%s err=%v", scope, err)
		}
	}
}

func TestMigrateUsesPackagedSchemaWithDevelopmentFallback(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "SyncAgent.exe")
	if got := migrationDirectory(exe, "server"); got != filepath.Join("migrations", "server") {
		t.Fatal(got)
	}
	want := filepath.Join(dir, "migrations", "server")
	if err := os.MkdirAll(want, 0700); err != nil {
		t.Fatal(err)
	}
	if got := migrationDirectory(exe, "server"); got != want {
		t.Fatalf("got=%s want=%s", got, want)
	}
}
