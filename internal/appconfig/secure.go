package appconfig

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const encryptedPrefix = "dpapi:"

type SecretProtector interface {
	Protect(plain string) (string, error)
	Unprotect(cipher string) (string, error)
}

func DefaultConfigPath() string {
	if override := strings.TrimSpace(os.Getenv("NODEBRIDGE_CONFIG_PATH")); override != "" {
		return override
	}
	root := strings.TrimSpace(os.Getenv("ProgramData"))
	if root == "" {
		root = "."
	}
	return filepath.Join(root, "NodeBridge", "config.yaml")
}

func SaveFile(path string, cfg Config) error {
	return SaveFileWithProtector(path, cfg, DefaultSecretProtector())
}

func SaveFileWithProtector(path string, cfg Config, protector SecretProtector) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if protector == nil {
		protector = plainProtector{}
	}
	if err := EncryptSecrets(&cfg, protector); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}

func MergeRedactedSecrets(next, previous Config) Config {
	if next.MySQL.Password == RedactedSecret {
		next.MySQL.Password = previous.MySQL.Password
	}
	if next.RabbitMQ.Password == RedactedSecret {
		next.RabbitMQ.Password = previous.RabbitMQ.Password
	}
	if isRedactedURL(next.RabbitMQ.LocalURL) {
		next.RabbitMQ.LocalURL = previous.RabbitMQ.LocalURL
	}
	if isRedactedURL(next.RabbitMQ.ServerURL) {
		next.RabbitMQ.ServerURL = previous.RabbitMQ.ServerURL
	}
	if next.CDC.Password == RedactedSecret {
		next.CDC.Password = previous.CDC.Password
	}
	if next.LogWeb.Token == RedactedSecret {
		next.LogWeb.Token = previous.LogWeb.Token
	}
	if next.Security.AdminPassword == RedactedSecret {
		next.Security.AdminPassword = previous.Security.AdminPassword
	}
	if next.Security.ExitPassword == RedactedSecret {
		next.Security.ExitPassword = previous.Security.ExitPassword
	}
	return next
}

func EncryptSecrets(cfg *Config, protector SecretProtector) error {
	var err error
	if cfg.MySQL.Password, err = protectSecret(cfg.MySQL.Password, protector); err != nil {
		return fmt.Errorf("encrypt mysql password: %w", err)
	}
	if cfg.RabbitMQ.Password, err = protectSecret(cfg.RabbitMQ.Password, protector); err != nil {
		return fmt.Errorf("encrypt rabbitmq password: %w", err)
	}
	if cfg.RabbitMQ.LocalURL, err = protectURL(cfg.RabbitMQ.LocalURL, protector); err != nil {
		return fmt.Errorf("encrypt rabbitmq local_url: %w", err)
	}
	if cfg.RabbitMQ.ServerURL, err = protectURL(cfg.RabbitMQ.ServerURL, protector); err != nil {
		return fmt.Errorf("encrypt rabbitmq server_url: %w", err)
	}
	if cfg.CDC.Password, err = protectSecret(cfg.CDC.Password, protector); err != nil {
		return fmt.Errorf("encrypt cdc password: %w", err)
	}
	if cfg.LogWeb.Token, err = protectSecret(cfg.LogWeb.Token, protector); err != nil {
		return fmt.Errorf("encrypt log web token: %w", err)
	}
	if cfg.Security.AdminPassword, err = protectSecret(cfg.Security.AdminPassword, protector); err != nil {
		return fmt.Errorf("encrypt admin password: %w", err)
	}
	if cfg.Security.ExitPassword, err = protectSecret(cfg.Security.ExitPassword, protector); err != nil {
		return fmt.Errorf("encrypt exit password: %w", err)
	}
	return nil
}

func DecryptSecrets(cfg *Config, protector SecretProtector) error {
	var err error
	if cfg.MySQL.Password, err = unprotectSecret(cfg.MySQL.Password, protector); err != nil {
		return fmt.Errorf("decrypt mysql password: %w", err)
	}
	if cfg.RabbitMQ.Password, err = unprotectSecret(cfg.RabbitMQ.Password, protector); err != nil {
		return fmt.Errorf("decrypt rabbitmq password: %w", err)
	}
	if cfg.RabbitMQ.LocalURL, err = unprotectSecret(cfg.RabbitMQ.LocalURL, protector); err != nil {
		return fmt.Errorf("decrypt rabbitmq local_url: %w", err)
	}
	if cfg.RabbitMQ.ServerURL, err = unprotectSecret(cfg.RabbitMQ.ServerURL, protector); err != nil {
		return fmt.Errorf("decrypt rabbitmq server_url: %w", err)
	}
	if cfg.CDC.Password, err = unprotectSecret(cfg.CDC.Password, protector); err != nil {
		return fmt.Errorf("decrypt cdc password: %w", err)
	}
	if cfg.LogWeb.Token, err = unprotectSecret(cfg.LogWeb.Token, protector); err != nil {
		return fmt.Errorf("decrypt log web token: %w", err)
	}
	if cfg.Security.AdminPassword, err = unprotectSecret(cfg.Security.AdminPassword, protector); err != nil {
		return fmt.Errorf("decrypt admin password: %w", err)
	}
	if cfg.Security.ExitPassword, err = unprotectSecret(cfg.Security.ExitPassword, protector); err != nil {
		return fmt.Errorf("decrypt exit password: %w", err)
	}
	return nil
}

func protectSecret(value string, protector SecretProtector) (string, error) {
	if strings.TrimSpace(value) == "" || strings.HasPrefix(value, encryptedPrefix) {
		return value, nil
	}
	cipher, err := protector.Protect(value)
	if err != nil {
		return "", err
	}
	return encryptedPrefix + cipher, nil
}

func protectURL(raw string, protector SecretProtector) (string, error) {
	if strings.TrimSpace(raw) == "" || strings.HasPrefix(raw, encryptedPrefix) {
		return raw, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return raw, nil
	}
	if _, ok := parsed.User.Password(); !ok {
		return raw, nil
	}
	return protectSecret(raw, protector)
}

func unprotectSecret(value string, protector SecretProtector) (string, error) {
	if !strings.HasPrefix(value, encryptedPrefix) {
		return value, nil
	}
	return protector.Unprotect(strings.TrimPrefix(value, encryptedPrefix))
}

func isRedactedURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return false
	}
	password, ok := parsed.User.Password()
	return ok && password == RedactedSecret
}

type plainProtector struct{}

func (plainProtector) Protect(plain string) (string, error) {
	return plain, nil
}

func (plainProtector) Unprotect(cipher string) (string, error) {
	return cipher, nil
}
