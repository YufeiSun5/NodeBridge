package uiapi

type FeatureCapability struct {
	ID        string   `json:"id"`
	Supported bool     `json:"supported"`
	Modes     []string `json:"modes,omitempty"`
	Message   string   `json:"message,omitempty"`
}

type ToolCapability struct {
	Name          string `json:"name"`
	RequiredScope string `json:"required_scope,omitempty"`
	Available     bool   `json:"available"`
	ReadOnly      bool   `json:"read_only"`
}

type CapabilitiesResponse struct {
	Version       string              `json:"version"`
	Transport     string              `json:"transport"`
	Features      []FeatureCapability `json:"features"`
	Tools         []ToolCapability    `json:"tools,omitempty"`
	LabFullAccess bool                `json:"lab_full_access"`
}
