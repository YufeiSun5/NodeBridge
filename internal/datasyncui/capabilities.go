package datasyncui

import "github.com/YufeiSun5/NodeBridge/internal/uiapi"

func (a *App) GetCapabilities() uiapi.CapabilitiesResponse {
	return uiapi.CapabilitiesResponse{Version: appVersion, Transport: "stdio", LabFullAccess: true, Features: []uiapi.FeatureCapability{
		{ID: "mcp.full_access", Supported: true},
		{ID: "sync.binary_values", Supported: true, Modes: []string{"tagged_base64"}, Message: "Canal binary columns retain exact bytes. All participating nodes must support the binary envelope."},
		{ID: "sync.queue_event_quarantine", Supported: true, Modes: []string{"plan", "apply", "audit"}, Message: "Bounded saved-rule event quarantine with durable copy before source ACK; not successful business application."},
		{ID: "sync.delete_mode", Supported: true, Modes: []string{"HARD", "SOFT"}, Message: "Missing legacy value means SOFT; HARD does not delete extra target rows."},
		{ID: "sync.conflict_policy", Supported: true, Modes: []string{"NONE", "LAST_WRITE_WIN"}, Message: "LAST_WRITE_WIN requires an aligned topology and uses source binlog seconds with a deterministic tie-break, not receipt time. Source clock regression may supersede later-issued commands."},
		{ID: "sync.bidirectional", Supported: true, Modes: []string{"aligned_pair", "aligned_star"}, Message: "Edge1-Server-EdgeN with reversible per-member mappings and a complete durable topology certificate. Three production Agents verified. No SERVER_WIN or online membership changes; existing conflict history cannot be snapshot-copied to new members."},
		{ID: "sync.rule_preflight", Supported: true, Modes: []string{"source", "target"}, Message: "Local schema and non-executing permissions; remote compatibility, CDC coverage and existing conflicts are explicitly unverified."},
		{ID: "sync.rule_revision", Supported: true, Modes: []string{"CAS", "active_revision"}},
		{ID: "sync.event_status", Supported: true, Message: "Bounded runtime error observations and apply receipts; not a consistency proof."},
		{ID: "initial_alignment.policy", Supported: true, Modes: []string{"DISABLED", "MANUAL"}},
		{ID: "initial_alignment.execute", Supported: true, Modes: []string{"manual_pair", "manual_topology", "empty_target", "both_empty", "mcp"}, Message: "Wails, CLI or MCP on all stopped participants. Server supplies every Edge ID; Edge supplies one Server ID. Raw limit 256 MiB, encoded limit 512 MiB, 15 minutes per copy; 200 MiB verified. Large snapshots require adequate Canal heap and unacknowledged-event buffer (managed defaults: 2 GiB heap, 512 MiB buffer). Committed copies reconcile on retry; no merge of populated endpoints or committed-data rollback."},
		{ID: "initial_alignment.online", Supported: false},
	}}
}
