package appconfig_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

func TestEncryptDecryptSecretsProtectsSensitiveFields(t *testing.T) {
	cfg := secureTestConfig()
	protector := testProtector{}

	if err := appconfig.EncryptSecrets(&cfg, protector); err != nil {
		t.Fatalf("EncryptSecrets returned error: %v", err)
	}
	for name, value := range map[string]string{
		"mysql":          cfg.MySQL.Password,
		"rabbitmq":       cfg.RabbitMQ.Password,
		"local_url":      cfg.RabbitMQ.LocalURL,
		"server_url":     cfg.RabbitMQ.ServerURL,
		"cdc":            cfg.CDC.Password,
		"log_web":        cfg.LogWeb.Token,
		"admin_password": cfg.Security.AdminPassword,
		"exit_password":  cfg.Security.ExitPassword,
	} {
		if !strings.HasPrefix(value, "dpapi:") {
			t.Fatalf("%s was not protected: %q", name, value)
		}
		if strings.Contains(value, "secret") || strings.Contains(value, "admin-pass") || strings.Contains(value, "exit-pass") {
			t.Fatalf("%s contains plaintext secret: %q", name, value)
		}
	}

	if err := appconfig.DecryptSecrets(&cfg, protector); err != nil {
		t.Fatalf("DecryptSecrets returned error: %v", err)
	}
	if cfg.MySQL.Password != "secret" || cfg.Security.AdminPassword != "admin-pass" ||
		cfg.Security.ExitPassword != "exit-pass" {
		t.Fatalf("unexpected decrypted config %+v", cfg)
	}
	if cfg.RabbitMQ.ServerURL != "amqp://sync:secret@127.0.0.1:5672/server-sync" {
		t.Fatalf("unexpected decrypted url %q", cfg.RabbitMQ.ServerURL)
	}
}

func TestSaveFileWithProtectorWritesEncryptedYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := appconfig.SaveFileWithProtector(path, secureTestConfig(), testProtector{}); err != nil {
		t.Fatalf("SaveFileWithProtector returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "dpapi:") {
		t.Fatalf("expected encrypted markers, got:\n%s", text)
	}
	if strings.Contains(text, "secret") || strings.Contains(text, "admin-pass") || strings.Contains(text, "exit-pass") {
		t.Fatalf("saved config contains plaintext secret:\n%s", text)
	}
}

func TestMergeRedactedSecretsPreservesExistingSensitiveValues(t *testing.T) {
	previous := secureTestConfig()
	next := previous
	next.Node.Name = "changed"
	next.MySQL.Password = appconfig.RedactedSecret
	next.RabbitMQ.Password = appconfig.RedactedSecret
	next.RabbitMQ.LocalURL = "amqp://sync:******@127.0.0.1:5672/edge-sync"
	next.RabbitMQ.ServerURL = "amqp://sync:******@127.0.0.1:5672/server-sync"
	next.CDC.Password = appconfig.RedactedSecret
	next.LogWeb.Token = appconfig.RedactedSecret
	next.Security.AdminPassword = appconfig.RedactedSecret
	next.Security.ExitPassword = appconfig.RedactedSecret

	merged := appconfig.MergeRedactedSecrets(next, previous)
	if merged.Node.Name != "changed" {
		t.Fatalf("expected non-secret field to change, got %+v", merged.Node)
	}
	if merged.MySQL.Password != previous.MySQL.Password ||
		merged.RabbitMQ.LocalURL != previous.RabbitMQ.LocalURL ||
		merged.Security.AdminPassword != previous.Security.AdminPassword ||
		merged.Security.ExitPassword != previous.Security.ExitPassword {
		t.Fatalf("expected secrets to be preserved, got %+v", merged)
	}
}

func secureTestConfig() appconfig.Config {
	return appconfig.Config{
		Mode: appconfig.ModeEdge,
		Node: appconfig.NodeConfig{ID: "edge-001", Name: "Edge"},
		MySQL: appconfig.MySQLConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			Username: "sync",
			Password: "secret",
			Database: "scada_edge",
		},
		RabbitMQ: appconfig.RabbitMQConfig{
			LocalURL:  "amqp://sync:secret@127.0.0.1:5672/edge-sync",
			ServerURL: "amqp://sync:secret@127.0.0.1:5672/server-sync",
			Password:  "secret",
		},
		CDC:      appconfig.CDCConfig{Type: "stub", Password: "secret"},
		Sync:     appconfig.SyncConfig{RetryIntervalSeconds: 5},
		LogWeb:   appconfig.LogWebConfig{Token: "secret"},
		Security: appconfig.SecurityConfig{AdminPassword: "admin-pass", ExitPassword: "exit-pass"},
	}
}

type testProtector struct{}

func (testProtector) Protect(plain string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(plain)), nil
}

func (testProtector) Unprotect(cipher string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(cipher)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
