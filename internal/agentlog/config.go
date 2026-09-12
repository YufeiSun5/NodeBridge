package agentlog

import (
	"net/url"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

func ForConfig(cfg *appconfig.Config) func(string) string {
	if cfg == nil {
		return Redactor()
	}
	secrets := []string{cfg.MySQL.Password, cfg.RabbitMQ.Password, cfg.CDC.Password, cfg.LogWeb.Token, cfg.Security.AdminPassword, cfg.Security.ExitPassword}
	for _, raw := range []string{cfg.RabbitMQ.LocalURL, cfg.RabbitMQ.ServerURL} {
		if parsed, err := url.Parse(raw); err == nil && parsed.User != nil {
			if password, ok := parsed.User.Password(); ok {
				secrets = append(secrets, password)
			}
		}
	}
	return Redactor(secrets...)
}
