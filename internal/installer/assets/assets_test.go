package assets

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCatalogChecksHashAndMissingAssets(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "otp_win64.exe")
	content := []byte("fake erlang installer")
	if err := os.WriteFile(assetPath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(content))

	results := ValidateCatalog(Catalog{Assets: []AssetSpec{
		{Name: "otp", Component: ComponentErlang, Path: assetPath, SHA256: sum},
		{Name: "rabbit", Component: ComponentRabbitMQ, Path: filepath.Join(dir, "missing.exe"), SHA256: strings.Repeat("0", 64)},
		{Name: "canal", Component: ComponentCanal, Path: assetPath, SHA256: strings.Repeat("1", 64)},
	}})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if !results[0].OK || results[0].Status != StatusOK || results[0].SizeBytes != int64(len(content)) {
		t.Fatalf("expected first asset ok: %+v", results[0])
	}
	if results[1].Status != StatusMissing {
		t.Fatalf("expected missing asset: %+v", results[1])
	}
	if results[2].Status != StatusHashMismatch {
		t.Fatalf("expected hash mismatch: %+v", results[2])
	}
	if AllValid(results) {
		t.Fatal("expected invalid catalog")
	}
}

func TestBuildCommandPlanDoesNotExecuteInstallers(t *testing.T) {
	plan := BuildCommandPlan(Catalog{Assets: []AssetSpec{
		{Name: "otp", Component: ComponentErlang, Path: `C:\packages\otp.exe`, SHA256: strings.Repeat("a", 64)},
		{Name: "rabbit", Component: ComponentRabbitMQ, Path: `C:\packages\rabbitmq.exe`, SHA256: strings.Repeat("b", 64)},
		{Name: "canal", Component: ComponentCanal, Path: `C:\packages\canal.zip`, SHA256: strings.Repeat("c", 64)},
	}})

	if len(plan) != 5 {
		t.Fatalf("expected 5 planned steps, got %d: %+v", len(plan), plan)
	}
	if plan[0].Component != ComponentErlang || plan[0].Action != "install" || !strings.Contains(plan[0].CommandLine, "/S") {
		t.Fatalf("unexpected erlang step: %+v", plan[0])
	}
	if plan[1].Component != ComponentRabbitMQ || plan[1].Action != "install" {
		t.Fatalf("unexpected rabbitmq install step: %+v", plan[1])
	}
	if plan[2].Action != "service-install" || plan[3].Action != "service-start" {
		t.Fatalf("expected rabbitmq service steps: %+v", plan[2:4])
	}
	if plan[4].Component != ComponentCanal || plan[4].Action != "extract" {
		t.Fatalf("unexpected canal step: %+v", plan[4])
	}
	for _, step := range plan {
		if step.Status != "planned" {
			t.Fatalf("command plan must stay planned only: %+v", step)
		}
	}
}

func TestLoadCatalogRequiresAssets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, []byte(`{"version":"test","assets":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCatalog(path); err == nil {
		t.Fatal("expected empty catalog error")
	}
}
