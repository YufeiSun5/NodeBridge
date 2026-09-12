package datasyncui

import "github.com/YufeiSun5/NodeBridge/internal/uiapi"

func (a *App) GetCapabilities() uiapi.CapabilitiesResponse {
	return uiapi.CapabilitiesResponse{Version: appVersion, Transport: "stdio", LabFullAccess: true, Features: []uiapi.FeatureCapability{
		{ID: "mcp.full_access", Supported: true},
		{ID: "sync.binary_values", Supported: true, Modes: []string{"tagged_base64"}, Message: "Canal binary columns retain exact bytes. All participating nodes must support the binary envelope."},
		{ID: "sync.queue_event_quarantine", Supported: true, Modes: []string{"plan", "apply", "audit"}, Message: "Bounded saved-rule event quarantine with durable copy before source ACK; not successful business application."},
		{ID: "sync.delete_mode", Supported: true, Modes: []string{"HARD", "SOFT"}, Message: "Missing legacy value means SOFT; HARD does not delete extra target rows."},
		{ID: "sync.conflict_policy", Supported: true, Modes: []string{"NONE"}},
		{ID: "sync.bidirectional", Supported: false},
		{ID: "sync.rule_preflight", Supported: true, Modes: []string{"source", "target"}, Message: "Local schema and non-executing permissions; remote compatibility, CDC coverage and existing conflicts are explicitly unverified."},
		{ID: "sync.rule_revision", Supported: true, Modes: []string{"CAS", "active_revision"}},
		{ID: "sync.event_status", Supported: true, Message: "Bounded runtime error observations and apply receipts; not a consistency proof."},
		{ID: "initial_alignment.policy", Supported: true, Modes: []string{"DISABLED", "MANUAL"}},
		{ID: "initial_alignment.execute", Supported: false},
		{ID: "initial_alignment.online", Supported: false},
	}}
}
