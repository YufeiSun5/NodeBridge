package status

const (
	AgentRunning = "running"
	AgentStopped = "stopped"
	AgentError   = "error"
)

type Overview struct {
	ProductName        string `json:"product_name"`
	Mode               string `json:"mode"`
	NodeID             string `json:"node_id,omitempty"`
	NodeName           string `json:"node_name,omitempty"`
	ConfigLoaded       bool   `json:"config_loaded"`
	ConfigPath         string `json:"config_path,omitempty"`
	RulesPath          string `json:"rules_path,omitempty"`
	AgentStatus        string `json:"agent_status"`
	AgentPID           int    `json:"agent_pid,omitempty"`
	AgentLogPath       string `json:"agent_log_path,omitempty"`
	MySQLStatus        string `json:"mysql_status,omitempty"`
	RabbitMQStatus     string `json:"rabbitmq_status,omitempty"`
	CDCStatus          string `json:"cdc_status,omitempty"`
	CDCMessage         string `json:"cdc_message,omitempty"`
	ServerStatus       string `json:"server_status,omitempty"`
	UploadQueueDepth   int64  `json:"upload_queue_depth"`
	DownlinkQueueDepth int64  `json:"downlink_queue_depth"`
	FailedEventCount   int64  `json:"failed_event_count"`
	ConflictCount      int64  `json:"conflict_count"`
	Version            string `json:"version"`
}
