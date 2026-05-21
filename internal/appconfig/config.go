package appconfig

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ModeUnknown = ""
	ModeEdge    = "edge"
	ModeServer  = "server"
)

type Config struct {
	Mode     string         `json:"mode" yaml:"mode"`
	Node     NodeConfig     `json:"node" yaml:"node"`
	MySQL    MySQLConfig    `json:"mysql" yaml:"mysql"`
	RabbitMQ RabbitMQConfig `json:"rabbitmq" yaml:"rabbitmq"`
	CDC      CDCConfig      `json:"cdc" yaml:"cdc"`
	Sync     SyncConfig     `json:"sync" yaml:"sync"`
	LogWeb   LogWebConfig   `json:"log_web" yaml:"log_web"`
}

type NodeConfig struct {
	ID       string `json:"id" yaml:"id"`
	Name     string `json:"name" yaml:"name"`
	Location string `json:"location,omitempty" yaml:"location,omitempty"`
}

type MySQLConfig struct {
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Database string `json:"database" yaml:"database"`
}

type RabbitMQConfig struct {
	Mode          string `json:"mode,omitempty" yaml:"mode,omitempty"`
	Install       bool   `json:"install,omitempty" yaml:"install,omitempty"`
	LocalURL      string `json:"local_url,omitempty" yaml:"local_url,omitempty"`
	ServerURL     string `json:"server_url" yaml:"server_url"`
	ManagementURL string `json:"management_url,omitempty" yaml:"management_url,omitempty"`
	Username      string `json:"username,omitempty" yaml:"username,omitempty"`
	Password      string `json:"password,omitempty" yaml:"password,omitempty"`
	VHost         string `json:"vhost,omitempty" yaml:"vhost,omitempty"`
}

type CDCConfig struct {
	Type        string `json:"type" yaml:"type"`
	CanalAddr   string `json:"canal_addr,omitempty" yaml:"canal_addr,omitempty"`
	Destination string `json:"destination,omitempty" yaml:"destination,omitempty"`
	BatchSize   int    `json:"batch_size,omitempty" yaml:"batch_size,omitempty"`
}

type SyncConfig struct {
	UploadBatchSize         int `json:"upload_batch_size,omitempty" yaml:"upload_batch_size,omitempty"`
	DispatchBatchSize       int `json:"dispatch_batch_size,omitempty" yaml:"dispatch_batch_size,omitempty"`
	RetryIntervalSeconds    int `json:"retry_interval_seconds" yaml:"retry_interval_seconds"`
	HeartbeatIntervalSecond int `json:"heartbeat_interval_seconds,omitempty" yaml:"heartbeat_interval_seconds,omitempty"`
	NodeTimeoutSeconds      int `json:"node_timeout_seconds,omitempty" yaml:"node_timeout_seconds,omitempty"`
}

type LogWebConfig struct {
	Enable bool   `json:"enable" yaml:"enable"`
	Bind   string `json:"bind" yaml:"bind"`
	Port   int    `json:"port" yaml:"port"`
	Token  string `json:"token" yaml:"token"`
}

func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config %q: %w", path, err)
	}
	return &cfg, nil
}

func (c Config) Validate() error {
	var problems []string

	switch strings.ToLower(strings.TrimSpace(c.Mode)) {
	case ModeEdge, ModeServer:
	case "":
		problems = append(problems, "mode is required")
	default:
		problems = append(problems, "mode must be edge or server")
	}

	if strings.TrimSpace(c.Node.ID) == "" {
		problems = append(problems, "node.id is required")
	}
	if strings.TrimSpace(c.MySQL.Database) == "" {
		problems = append(problems, "mysql.database is required")
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
