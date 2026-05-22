package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	canalplan "github.com/YufeiSun5/NodeBridge/internal/installer/canal"
	"github.com/YufeiSun5/NodeBridge/internal/installer/manifest"
	rabbitplan "github.com/YufeiSun5/NodeBridge/internal/installer/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
)

const (
	ModePlan      = "plan"
	ModeApply     = "apply"
	ModeRepair    = "repair"
	ModeUninstall = "uninstall"

	StatusPlanned = "planned"
	StatusDone    = "done"
	StatusSkipped = "skipped"
	StatusError   = "error"
)

type Request struct {
	Config       appconfig.Config `json:"config"`
	ConfigPath   string           `json:"config_path,omitempty"`
	ManifestPath string           `json:"manifest_path"`
	Version      string           `json:"version,omitempty"`
	InstallID    string           `json:"install_id,omitempty"`
}

type Operation struct {
	Component string `json:"component"`
	Action    string `json:"action"`
	Target    string `json:"target,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

type Result struct {
	Mode         string            `json:"mode"`
	ManifestPath string            `json:"manifest_path"`
	Operations   []Operation       `json:"operations"`
	Manifest     manifest.Manifest `json:"manifest"`
}

type Executor struct {
	Now            func() time.Time
	WriteFile      func(string, []byte, os.FileMode) error
	MkdirAll       func(string, os.FileMode) error
	RemoveAll      func(string) error
	SaveManifest   func(string, manifest.Manifest) error
	InitRabbitMQ   func(context.Context, appconfig.Config) error
	LoadManifest   func(string) (manifest.Manifest, error)
	WriteCanalFile func(appconfig.Config, manifest.Manifest) (string, error)
}

func New() Executor {
	return Executor{
		Now:            time.Now,
		WriteFile:      os.WriteFile,
		MkdirAll:       os.MkdirAll,
		RemoveAll:      os.RemoveAll,
		SaveManifest:   manifest.Save,
		InitRabbitMQ:   initRabbitMQ,
		LoadManifest:   manifest.Load,
		WriteCanalFile: writeCanalDestinationConfig,
	}
}

func (e Executor) Plan(req Request) Result {
	m := buildManifest(req, e.now())
	return Result{Mode: ModePlan, ManifestPath: req.ManifestPath, Manifest: m, Operations: plannedOperations(req, m)}
}

func (e Executor) Apply(ctx context.Context, req Request) (Result, error) {
	result := e.Plan(req)
	result.Mode = ModeApply
	for i := range result.Operations {
		result.Operations[i].Status = StatusDone
		switch result.Operations[i].Component + ":" + result.Operations[i].Action {
		case "manifest:write":
			if err := e.saveManifest(req.ManifestPath, result.Manifest); err != nil {
				return fail(result, i, err), err
			}
		case "canal-config:write":
			target, err := e.writeCanalFile(req.Config, result.Manifest)
			if err != nil {
				return fail(result, i, err), err
			}
			result.Operations[i].Target = target
		case "rabbitmq-topology:initialize":
			if err := e.initRabbitMQ(ctx, req.Config); err != nil {
				return fail(result, i, err), err
			}
		default:
			result.Operations[i].Status = StatusSkipped
		}
	}
	return result, nil
}

func (e Executor) Repair(ctx context.Context, req Request) (Result, error) {
	result, err := e.Apply(ctx, req)
	result.Mode = ModeRepair
	return result, err
}

func (e Executor) Uninstall(ctx context.Context, req Request) (Result, error) {
	_ = ctx
	m, err := e.loadManifest(req.ManifestPath)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		Mode:         ModeUninstall,
		ManifestPath: req.ManifestPath,
		Manifest:     m,
		Operations: []Operation{
			{Component: "canal-config", Action: "remove", Target: m.ManagedComponents.Canal.ConfigDir, Status: StatusPlanned},
			{Component: "manifest", Action: "remove", Target: req.ManifestPath, Status: StatusPlanned},
		},
	}
	for i := range result.Operations {
		result.Operations[i].Status = StatusDone
		switch result.Operations[i].Component {
		case "canal-config":
			if m.ManagedComponents.Canal.Mode == manifest.ModeManaged && strings.Contains(strings.ToLower(m.ManagedComponents.Canal.ConfigDir), "nodebridge") {
				if err := e.removeAll(expandProgramData(m.ManagedComponents.Canal.ConfigDir)); err != nil {
					return fail(result, i, err), err
				}
			} else {
				result.Operations[i].Status = StatusSkipped
			}
		case "manifest":
			if err := e.removeAll(req.ManifestPath); err != nil {
				return fail(result, i, err), err
			}
		}
	}
	return result, nil
}

func plannedOperations(req Request, m manifest.Manifest) []Operation {
	var ops []Operation
	rabbitDesired := rabbitDesiredState(req.Config, m)
	for _, step := range rabbitplan.BuildPlan(rabbitplan.CurrentState{}, rabbitDesired).Steps {
		ops = append(ops, Operation{Component: "rabbitmq-" + step.Component, Action: step.Action, Target: step.Target, Status: StatusPlanned})
	}
	if rabbitDesired.Mode == manifest.ModeManaged && rabbitURL(req.Config) != "" {
		ops = append(ops, Operation{Component: "rabbitmq-topology", Action: "initialize", Target: "configured-amqp-url", Status: StatusPlanned})
	}

	canalDesired := canalDesiredState(req.Config, m)
	for _, step := range canalplan.BuildPlan(canalplan.CurrentState{}, canalDesired).Steps {
		ops = append(ops, Operation{Component: "canal-" + step.Component, Action: step.Action, Target: step.Target, Status: StatusPlanned})
	}
	if canalDesired.Mode == manifest.ModeManaged {
		ops = append(ops, Operation{Component: "canal-config", Action: "write", Target: canalDestinationPath(canalDesired.ConfigDir, canalDestination(req.Config)), Status: StatusPlanned})
	}
	ops = append(ops, Operation{Component: "manifest", Action: "write", Target: req.ManifestPath, Status: StatusPlanned})
	return ops
}

func buildManifest(req Request, now time.Time) manifest.Manifest {
	installID := strings.TrimSpace(req.InstallID)
	if installID == "" {
		installID = "nodebridge-local"
	}
	m := manifest.New(installID, req.Version, now)
	if mode := normalizedMode(req.Config.RabbitMQ.Mode, req.Config.RabbitMQ.Install); mode != "" {
		m.ManagedComponents.RabbitMQ.Mode = mode
	}
	if mode := normalizedMode(req.Config.CDC.Mode, req.Config.CDC.Install); mode != "" {
		m.ManagedComponents.Canal.Mode = mode
	}
	if req.Config.CDC.ServiceName != "" {
		m.ManagedComponents.Canal.ServiceName = req.Config.CDC.ServiceName
	}
	if req.Config.CDC.ConfigDir != "" {
		m.ManagedComponents.Canal.ConfigDir = req.Config.CDC.ConfigDir
	}
	if destination := canalDestination(req.Config); destination != "" {
		m.ManagedComponents.Canal.Destinations = []string{destination}
	}
	return m
}

func rabbitDesiredState(cfg appconfig.Config, m manifest.Manifest) rabbitplan.DesiredState {
	desired := rabbitplan.DefaultDesiredState()
	desired.Mode = m.ManagedComponents.RabbitMQ.Mode
	desired.ServiceName = m.ManagedComponents.RabbitMQ.ServiceName
	desired.VHosts = append([]string(nil), m.ManagedComponents.RabbitMQ.VHosts...)
	return desired
}

func canalDesiredState(cfg appconfig.Config, m manifest.Manifest) canalplan.DesiredState {
	desired := canalplan.DefaultDesiredState(cfg.Node.ID)
	desired.Mode = m.ManagedComponents.Canal.Mode
	desired.ServiceName = m.ManagedComponents.Canal.ServiceName
	desired.ConfigDir = m.ManagedComponents.Canal.ConfigDir
	desired.Destinations = []canalplan.DestinationSpec{{
		Name:          canalDestination(cfg),
		MySQLHost:     cfg.MySQL.Host,
		MySQLPort:     cfg.MySQL.Port,
		MySQLUsername: cfg.MySQL.Username,
		Filter:        cfg.CDC.Filter,
	}}
	return desired
}

func initRabbitMQ(ctx context.Context, cfg appconfig.Config) error {
	_ = ctx
	url := rabbitURL(cfg)
	if url == "" {
		return nil
	}
	conn, err := rabbitmq.Dial(url)
	if err != nil {
		return err
	}
	defer conn.Close()
	topology := rabbitmq.EdgeTopology()
	if cfg.Mode == appconfig.ModeServer {
		topology = rabbitmq.ServerTopology(nil)
	}
	return rabbitmq.InitializeTopology(conn.Channel, topology)
}

func writeCanalDestinationConfig(cfg appconfig.Config, m manifest.Manifest) (string, error) {
	dir := canalDestinationPath(m.ManagedComponents.Canal.ConfigDir, canalDestination(cfg))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	content := fmt.Sprintf(`# NodeBridge managed. / NodeBridge 管理。 / NodeBridge 管理。
canal.instance.master.address=%s:%d
canal.instance.dbUsername=%s
canal.instance.filter.regex=%s
`, cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Username, defaultString(cfg.CDC.Filter, ".*\\..*"))
	path := filepath.Join(dir, "instance.properties")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func canalDestinationPath(configDir, destination string) string {
	return filepath.Join(expandProgramData(configDir), destination)
}

func expandProgramData(path string) string {
	if strings.HasPrefix(path, `%ProgramData%`) {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, strings.TrimPrefix(path, `%ProgramData%`))
	}
	return path
}

func canalDestination(cfg appconfig.Config) string {
	if cfg.CDC.Destination != "" {
		if strings.HasPrefix(cfg.CDC.Destination, "nodebridge-") {
			return cfg.CDC.Destination
		}
		return "nodebridge-" + cfg.CDC.Destination
	}
	if cfg.Node.ID == "" {
		return "nodebridge-local"
	}
	return "nodebridge-" + cfg.Node.ID
}

func rabbitURL(cfg appconfig.Config) string {
	if cfg.Mode == appconfig.ModeEdge && cfg.RabbitMQ.LocalURL != "" {
		return cfg.RabbitMQ.LocalURL
	}
	return cfg.RabbitMQ.ServerURL
}

func normalizedMode(mode string, install bool) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case manifest.ModeManaged, "":
		if install || strings.TrimSpace(mode) == manifest.ModeManaged {
			return manifest.ModeManaged
		}
	case manifest.ModeExternal:
		return manifest.ModeExternal
	}
	return ""
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (e Executor) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e Executor) saveManifest(path string, m manifest.Manifest) error {
	if e.SaveManifest != nil {
		return e.SaveManifest(path, m)
	}
	return manifest.Save(path, m)
}

func (e Executor) loadManifest(path string) (manifest.Manifest, error) {
	if e.LoadManifest != nil {
		return e.LoadManifest(path)
	}
	return manifest.Load(path)
}

func (e Executor) initRabbitMQ(ctx context.Context, cfg appconfig.Config) error {
	if e.InitRabbitMQ != nil {
		return e.InitRabbitMQ(ctx, cfg)
	}
	return initRabbitMQ(ctx, cfg)
}

func (e Executor) writeCanalFile(cfg appconfig.Config, m manifest.Manifest) (string, error) {
	if e.WriteCanalFile != nil {
		return e.WriteCanalFile(cfg, m)
	}
	return writeCanalDestinationConfig(cfg, m)
}

func (e Executor) removeAll(path string) error {
	if e.RemoveAll != nil {
		return e.RemoveAll(path)
	}
	return os.RemoveAll(path)
}

func fail(result Result, index int, err error) Result {
	result.Operations[index].Status = StatusError
	result.Operations[index].Message = err.Error()
	return result
}

func JSON(result Result) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}
