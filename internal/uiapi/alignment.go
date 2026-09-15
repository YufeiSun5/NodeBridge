package uiapi

type InitialAlignmentRequest struct {
	RuleID     string `json:"rule_id"`
	PeerNodeID string `json:"peer_node_id"`
	Confirm    bool   `json:"confirm"`
}

type InitialAlignmentStatus struct {
	RuleID     string `json:"rule_id"`
	PeerNodeID string `json:"peer_node_id"`
	Running    bool   `json:"running"`
	Stage      string `json:"stage"`
	PlanID     string `json:"plan_id,omitempty"`
	Rows       int64  `json:"rows"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}
