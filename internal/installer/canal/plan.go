package canal

const (
	ComponentCanal       = "canal"
	ComponentCanalConfig = "canal-config"
	ComponentService     = "canal-service"

	ActionInstall    = "install"
	ActionCreate     = "create"
	ActionStart      = "start"
	ActionNoop       = "noop"
	ActionPreserve   = "preserve"
	ActionInitialize = "initialize"
)

type DesiredState struct {
	Mode              string
	CanalArchivePath  string
	ServiceName       string
	ConfigDir         string
	Destinations      []DestinationSpec
	PreserveData      bool
	ManagedResourceID string
}

type DestinationSpec struct {
	Name          string
	MySQLHost     string
	MySQLPort     int
	MySQLUsername string
	Filter        string
}

type CurrentState struct {
	CanalInstalled bool
	ServiceRunning bool
	ConfigDirs     map[string]bool
	Destinations   map[string]bool
	OwnedResources map[string]bool
}

type Step struct {
	Component string
	Action    string
	Target    string
}

type Plan struct {
	Steps []Step
}

func BuildPlan(current CurrentState, desired DesiredState) Plan {
	if desired.Mode == "external" {
		return Plan{Steps: []Step{{Component: ComponentCanal, Action: ActionNoop, Target: "external-canal"}}}
	}

	var steps []Step
	if !current.CanalInstalled {
		steps = append(steps, Step{Component: ComponentCanal, Action: ActionInstall, Target: desired.CanalArchivePath})
	}
	if !current.ConfigDirs[desired.ConfigDir] {
		steps = append(steps, Step{Component: ComponentCanalConfig, Action: ActionCreate, Target: desired.ConfigDir})
	}
	for _, destination := range desired.Destinations {
		if !current.Destinations[destination.Name] {
			steps = append(steps, Step{Component: ComponentCanalConfig, Action: ActionInitialize, Target: "destination:" + destination.Name})
		}
	}
	if !current.ServiceRunning {
		steps = append(steps, Step{Component: ComponentService, Action: ActionStart, Target: desired.ServiceName})
	}
	if desired.PreserveData {
		steps = append(steps, Step{Component: ComponentCanal, Action: ActionPreserve, Target: "data-dir"})
	}
	if len(steps) == 0 {
		steps = append(steps, Step{Component: ComponentCanal, Action: ActionNoop, Target: "already-ready"})
	}
	return Plan{Steps: steps}
}

func DefaultDesiredState(nodeID string) DesiredState {
	if nodeID == "" {
		nodeID = "edge-001"
	}
	return DesiredState{
		Mode:              "managed",
		CanalArchivePath:  "deploy/windows/canal-server.zip",
		ServiceName:       "NodeBridgeCanal",
		ConfigDir:         `%ProgramData%\NodeBridge\canal`,
		ManagedResourceID: "nodebridge",
		Destinations: []DestinationSpec{
			{
				Name:          "nodebridge-" + nodeID,
				MySQLHost:     "127.0.0.1",
				MySQLPort:     3306,
				MySQLUsername: "sync_user",
				Filter:        ".*\\..*",
			},
		},
		PreserveData: true,
	}
}
