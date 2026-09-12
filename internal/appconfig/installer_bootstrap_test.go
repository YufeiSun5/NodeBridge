package appconfig_test

import (
	"path/filepath"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

func TestInstallerBootstrapModes(t *testing.T) {
	for _, tc := range []struct {
		file    string
		mode    string
		install bool
	}{
		{"installer-bootstrap.yaml", "managed", true},
		{"installer-external-bootstrap.yaml", "external", false},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			path := filepath.Join("..", "..", "configs", tc.file)
			cfg, err := appconfig.LoadFileAllowIncomplete(path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.RabbitMQ.Mode != tc.mode || cfg.CDC.Mode != tc.mode || cfg.RabbitMQ.Install != tc.install || cfg.CDC.Install != tc.install {
				t.Fatal("bootstrap component ownership differs from installer selection")
			}
			if cfg.Mode != "" || cfg.Node.ID != "" || cfg.MySQL.Database != "" || cfg.MySQL.Password != "" || cfg.MCP.Enable {
				t.Fatal("bootstrap must not preset business identity, credentials, or MCP access")
			}
			if _, err := appconfig.LoadFile(path); err == nil {
				t.Fatal("incomplete bootstrap must not be runnable")
			}
		})
	}
}
