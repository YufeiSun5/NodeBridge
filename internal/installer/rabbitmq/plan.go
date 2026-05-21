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
	ErlangInstallerPath   string
	RabbitMQInstallerPath string
	ServiceName           string
	VHosts                []string
	Users                 []UserSpec
	PreserveData          bool
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
		ErlangInstallerPath:   "deploy/windows/otp_win64.exe",
		RabbitMQInstallerPath: "deploy/windows/rabbitmq-server.exe",
		ServiceName:           "RabbitMQ",
		VHosts:                []string{"/edge-sync", "/server-sync"},
		Users: []UserSpec{
			{Username: "server-sync", VHost: "/server-sync", ConfigureRE: ".*", WriteRE: ".*", ReadRE: ".*"},
			{Username: "edge-001", VHost: "/server-sync", ConfigureRE: "^$", WriteRE: "server\\.ingress\\..*", ReadRE: "edge-001\\.downlink\\..*"},
			{Username: "edge-001-local", VHost: "/edge-sync", ConfigureRE: ".*", WriteRE: ".*", ReadRE: ".*"},
		},
		PreserveData: true,
	}
}
