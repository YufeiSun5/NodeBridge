package rabbitmq

const (
	ComponentErlang   = "erlang"
	ComponentRabbitMQ = "rabbitmq"
	ComponentService  = "rabbitmq-service"
	ComponentTopology = "rabbitmq-topology"

	ActionInstall      = "install"
	ActionStart        = "start"
	ActionCreate       = "create"
	ActionGrant        = "grant"
	ActionInitialize   = "initialize"
	ActionNoop         = "noop"
	ActionPreserveData = "preserve-data"
)

type DesiredState struct {
	Mode                  string
	ErlangInstallerPath   string
	RabbitMQInstallerPath string
	ServiceName           string
	VHosts                []string
	Users                 []UserSpec
	PreserveData          bool
	ManagedResourceID     string
}

type UserSpec struct {
	Username    string
	Password    string
	VHost       string
	ConfigureRE string
	WriteRE     string
	ReadRE      string
}

type CurrentState struct {
	ErlangInstalled   bool
	RabbitMQInstalled bool
	ServiceInstalled  bool
	ServiceRunning    bool
	VHosts            map[string]bool
	Users             map[string]bool
	Permissions       map[string]bool
	TopologyReady     bool
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
		return Plan{Steps: []Step{{Component: ComponentRabbitMQ, Action: ActionNoop, Target: "external-rabbitmq"}}}
	}

	var steps []Step

	if !current.ErlangInstalled {
		steps = append(steps, Step{Component: ComponentErlang, Action: ActionInstall, Target: desired.ErlangInstallerPath})
	}
	if !current.RabbitMQInstalled {
		steps = append(steps, Step{Component: ComponentRabbitMQ, Action: ActionInstall, Target: desired.RabbitMQInstallerPath})
	}
	if !current.ServiceInstalled {
		steps = append(steps, Step{Component: ComponentService, Action: ActionInstall, Target: desired.ServiceName})
	}
	if !current.ServiceRunning {
		steps = append(steps, Step{Component: ComponentService, Action: ActionStart, Target: desired.ServiceName})
	}

	for _, vhost := range desired.VHosts {
		if !current.VHosts[vhost] {
			steps = append(steps, Step{Component: ComponentRabbitMQ, Action: ActionCreate, Target: "vhost:" + vhost})
		}
	}

	for _, user := range desired.Users {
		if !current.Users[user.Username] {
			steps = append(steps, Step{Component: ComponentRabbitMQ, Action: ActionCreate, Target: "user:" + user.Username})
		}
		permissionKey := user.Username + "@" + user.VHost
		if !current.Permissions[permissionKey] {
			steps = append(steps, Step{Component: ComponentRabbitMQ, Action: ActionGrant, Target: permissionKey})
		}
	}

	if !current.TopologyReady {
		steps = append(steps, Step{Component: ComponentTopology, Action: ActionInitialize, Target: "edge/server"})
	}
	if desired.PreserveData {
		steps = append(steps, Step{Component: ComponentRabbitMQ, Action: ActionPreserveData, Target: "data-dir"})
	}
	if len(steps) == 0 {
		steps = append(steps, Step{Component: ComponentRabbitMQ, Action: ActionNoop, Target: "already-ready"})
	}

	return Plan{Steps: steps}
}

func DefaultDesiredState() DesiredState {
	return DesiredState{
		Mode:                  "managed",
		ErlangInstallerPath:   "deploy/windows/otp_win64.exe",
		RabbitMQInstallerPath: "deploy/windows/rabbitmq-server.exe",
		ServiceName:           "NodeBridgeRabbitMQ",
		VHosts:                []string{"/nodebridge-edge", "/nodebridge-server"},
		Users: []UserSpec{
			{Username: "nb-server-sync", VHost: "/nodebridge-server", ConfigureRE: ".*", WriteRE: ".*", ReadRE: ".*"},
			{Username: "nb-edge-001", VHost: "/nodebridge-server", ConfigureRE: "^$", WriteRE: "server\\.ingress\\..*", ReadRE: "edge-001\\.downlink\\..*"},
			{Username: "nb-edge-001-local", VHost: "/nodebridge-edge", ConfigureRE: ".*", WriteRE: ".*", ReadRE: ".*"},
		},
		PreserveData:      true,
		ManagedResourceID: "nodebridge",
	}
}
