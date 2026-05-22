package uiapi

import (
	"net/url"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/status"
)

const (
	StateUnknown     = "unknown"
	StateUnsupported = "unsupported"
	StateLocked      = "locked"
	StateUnlocked    = "unlocked"
	StateConfigured  = "configured"
)

type ConfigDTO struct {
	Mode     string                    `json:"mode"`
	Node     appconfig.NodeConfig      `json:"node"`
	MySQL    appconfig.MySQLConfig     `json:"mysql"`
	RabbitMQ appconfig.RabbitMQConfig  `json:"rabbitmq"`
	CDC      appconfig.CDCConfig       `json:"cdc"`
	Sync     appconfig.SyncConfig      `json:"sync"`
	LogWeb   appconfig.LogWebConfig    `json:"log_web"`
	MCP      appconfig.MCPServerConfig `json:"mcp_server,omitempty"`
	Security appconfig.SecurityConfig  `json:"security,omitempty"`
}

type SaveConfigRequest struct {
	Config ConfigDTO `json:"config"`
}

type SyncRulesDTO struct {
	Rules []rules.SyncRule `json:"rules"`
}

type SaveSyncRulesRequest struct {
	Rules []rules.SyncRule `json:"rules"`
}

type TestResult struct {
	OK      bool   `json:"ok"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type QueueStatusDTO struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	Messages  int    `json:"messages"`
	Consumers int    `json:"consumers"`
	Status    string `json:"status"`
}

type QueueStatusResponse struct {
	Queues []QueueStatusDTO `json:"queues"`
}

type FailedEventsRequest struct {
	Limit int `json:"limit"`
}

type FailedEventDTO struct {
	EventID      string `json:"event_id"`
	TargetNodeID string `json:"target_node_id"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	CreatedAt    string `json:"created_at,omitempty"`
}

type FailedEventsResponse struct {
	Items []FailedEventDTO `json:"items"`
}

type RetryFailedEventRequest struct {
	EventID      string `json:"event_id"`
	TargetNodeID string `json:"target_node_id"`
}

type RetryFailedEventsRequest struct {
	Limit int `json:"limit"`
}

type DeadLetterRequest struct {
	Queue string `json:"queue,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type DeadLetterMessageDTO struct {
	Queue       string            `json:"queue"`
	ContentType string            `json:"content_type,omitempty"`
	BodyPreview string            `json:"body_preview"`
	BodySize    int               `json:"body_size"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type DeadLetterResponse struct {
	Items []DeadLetterMessageDTO `json:"items"`
}

type LogQuery struct {
	Level  string `json:"level,omitempty"`
	Module string `json:"module,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Module  string `json:"module"`
	Message string `json:"message"`
}

type LogsResponse struct {
	Items []LogEntry `json:"items"`
}

type OperationResult struct {
	OK      bool   `json:"ok"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type UnlockAdminRequest struct {
	Password string `json:"password"`
}

type AuthState struct {
	Unlocked       bool   `json:"unlocked"`
	Status         string `json:"status"`
	ExpiresAt      string `json:"expires_at,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Message        string `json:"message,omitempty"`
}

type VerifyExitPasswordRequest struct {
	Password string `json:"password"`
}

type AutoStartStatus struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type SetAutoStartRequest struct {
	Enabled bool `json:"enabled"`
}

type MCPServerStatus struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type SetMCPServerEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

type ManagedInstallRequest struct {
	ManifestPath string `json:"manifest_path,omitempty"`
}

type ManagedInstallOperationDTO struct {
	Component string `json:"component"`
	Action    string `json:"action"`
	Target    string `json:"target,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

type ManagedInstallResponse struct {
	Mode         string                       `json:"mode"`
	ManifestPath string                       `json:"manifest_path"`
	Operations   []ManagedInstallOperationDTO `json:"operations"`
}

type AgentProcessStatus struct {
	ExecutablePath string `json:"executable_path,omitempty"`
	PID            int    `json:"pid,omitempty"`
	Status         string `json:"status"`
	StartedAt      string `json:"started_at,omitempty"`
	ExitedAt       string `json:"exited_at,omitempty"`
	LastError      string `json:"last_error,omitempty"`
	LogPath        string `json:"log_path,omitempty"`
}

type DiagnosticPackageResponse struct {
	Path string `json:"path"`
}

func ConfigFromApp(cfg appconfig.Config) ConfigDTO {
	return ConfigDTO{
		Mode:     cfg.Mode,
		Node:     cfg.Node,
		MySQL:    cfg.MySQL,
		RabbitMQ: cfg.RabbitMQ,
		CDC:      cfg.CDC,
		Sync:     cfg.Sync,
		LogWeb:   cfg.LogWeb,
		MCP:      cfg.MCP,
		Security: cfg.Security,
	}
}

func (c ConfigDTO) ToAppConfig() appconfig.Config {
	return appconfig.Config{
		Mode:     c.Mode,
		Node:     c.Node,
		MySQL:    c.MySQL,
		RabbitMQ: c.RabbitMQ,
		CDC:      c.CDC,
		Sync:     c.Sync,
		LogWeb:   c.LogWeb,
		MCP:      c.MCP,
		Security: c.Security,
	}
}

func RedactConfig(cfg ConfigDTO) ConfigDTO {
	cfg.MySQL.Password = redactSecret(cfg.MySQL.Password)
	cfg.RabbitMQ.Password = redactSecret(cfg.RabbitMQ.Password)
	cfg.RabbitMQ.LocalURL = RedactURL(cfg.RabbitMQ.LocalURL)
	cfg.RabbitMQ.ServerURL = RedactURL(cfg.RabbitMQ.ServerURL)
	cfg.CDC.Password = redactSecret(cfg.CDC.Password)
	cfg.LogWeb.Token = redactSecret(cfg.LogWeb.Token)
	cfg.Security.AdminPassword = redactSecret(cfg.Security.AdminPassword)
	cfg.Security.ExitPassword = redactSecret(cfg.Security.ExitPassword)
	return cfg
}

func RedactOverview(overview status.Overview) status.Overview {
	if overview.MySQLStatus == "" {
		overview.MySQLStatus = StateUnknown
	}
	if overview.RabbitMQStatus == "" {
		overview.RabbitMQStatus = StateUnknown
	}
	if overview.CDCStatus == "" {
		overview.CDCStatus = StateUnknown
	}
	return overview
}

func TimeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func RedactURL(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return raw
	}
	username := parsed.User.Username()
	if username == "" {
		return raw
	}
	parsed.User = url.UserPassword(username, "******")
	return parsed.String()
}

func redactSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return appconfig.RedactedSecret
}
