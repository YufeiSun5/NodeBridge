package uiapi_test

import (
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

func TestRedactConfig(t *testing.T) {
	cfg := uiapi.RedactConfig(uiapi.ConfigFromApp(appconfig.Config{
		MySQL: appconfig.MySQLConfig{Password: "secret"},
		RabbitMQ: appconfig.RabbitMQConfig{
			Password:  "mq-secret",
			LocalURL:  "amqp://sync:secret@127.0.0.1:5672/edge",
			ServerURL: "amqp://server:secret@127.0.0.1:5672/server",
		},
		CDC:    appconfig.CDCConfig{Password: "cdc-secret"},
		LogWeb: appconfig.LogWebConfig{Token: "token"},
	}))

	if cfg.MySQL.Password != "******" || cfg.CDC.Password != "******" || cfg.LogWeb.Token != "******" {
		t.Fatalf("secrets were not redacted: %+v", cfg)
	}
	if cfg.RabbitMQ.LocalURL != "amqp://sync:%2A%2A%2A%2A%2A%2A@127.0.0.1:5672/edge" {
		t.Fatalf("unexpected redacted url %q", cfg.RabbitMQ.LocalURL)
	}
}
