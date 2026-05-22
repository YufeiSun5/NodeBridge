package manifest_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/installer/manifest"
)

func TestManifestOwnership(t *testing.T) {
	m := manifest.New("install-001", "0.28.0", time.Unix(100, 0))

	if !m.OwnsRabbitMQVHost("/nodebridge-server") {
		t.Fatal("expected NodeBridge server vhost ownership")
	}
	if m.OwnsRabbitMQVHost("/customer") {
		t.Fatal("must not own customer vhost")
	}
	if !m.OwnsRabbitMQUser("nb-edge-001") {
		t.Fatal("expected NodeBridge user ownership")
	}
	if m.OwnsRabbitMQUser("guest") {
		t.Fatal("must not own guest user")
	}
	if !m.OwnsCanalDestination("nodebridge-edge-001") {
		t.Fatal("expected NodeBridge canal destination ownership")
	}
}

func TestSaveLoadManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install-manifest.json")
	want := manifest.New("install-001", "0.28.0", time.Unix(100, 0))

	if err := manifest.Save(path, want); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	got, err := manifest.Load(path)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if got.Product != manifest.ProductName || got.InstallID != "install-001" {
		t.Fatalf("unexpected manifest %+v", got)
	}
}

func TestValidateRejectsUnknownProduct(t *testing.T) {
	m := manifest.New("install-001", "0.28.0", time.Unix(100, 0))
	m.Product = "Other"

	if err := m.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
