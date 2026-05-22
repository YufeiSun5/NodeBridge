package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ProductName  = "NodeBridge"
	ModeManaged  = "managed"
	ModeExternal = "external"
)

type Manifest struct {
	Product           string            `json:"product"`
	InstallID         string            `json:"install_id"`
	Version           string            `json:"version,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	ManagedComponents ManagedComponents `json:"managed_components"`
}

type ManagedComponents struct {
	RabbitMQ RabbitMQComponent `json:"rabbitmq"`
	Canal    CanalComponent    `json:"canal"`
}

type RabbitMQComponent struct {
	Mode        string   `json:"mode"`
	ServiceName string   `json:"service_name,omitempty"`
	VHosts      []string `json:"vhosts,omitempty"`
	Users       []string `json:"users,omitempty"`
	TopologyTag string   `json:"topology_tag,omitempty"`
}

type CanalComponent struct {
	Mode         string   `json:"mode"`
	ServiceName  string   `json:"service_name,omitempty"`
	ConfigDir    string   `json:"config_dir,omitempty"`
	Destinations []string `json:"destinations,omitempty"`
}

func New(installID, version string, now time.Time) Manifest {
	if now.IsZero() {
		now = time.Now()
	}
	return Manifest{
		Product:   ProductName,
		InstallID: strings.TrimSpace(installID),
		Version:   strings.TrimSpace(version),
		CreatedAt: now,
		ManagedComponents: ManagedComponents{
			RabbitMQ: RabbitMQComponent{
				Mode:        ModeManaged,
				ServiceName: "NodeBridgeRabbitMQ",
				VHosts:      []string{"/nodebridge-edge", "/nodebridge-server"},
				Users:       []string{"nb-server-sync", "nb-edge-001", "nb-edge-001-local"},
				TopologyTag: "nodebridge",
			},
			Canal: CanalComponent{
				Mode:         ModeManaged,
				ServiceName:  "NodeBridgeCanal",
				ConfigDir:    `%ProgramData%\NodeBridge\canal`,
				Destinations: []string{"nodebridge-edge-001", "nodebridge-server-001"},
			},
		},
	}
}

func (m Manifest) Validate() error {
	var problems []string
	if m.Product != ProductName {
		problems = append(problems, "product must be NodeBridge")
	}
	if strings.TrimSpace(m.InstallID) == "" {
		problems = append(problems, "install_id is required")
	}
	if err := validateMode("rabbitmq.mode", m.ManagedComponents.RabbitMQ.Mode); err != nil {
		problems = append(problems, err.Error())
	}
	if err := validateMode("canal.mode", m.ManagedComponents.Canal.Mode); err != nil {
		problems = append(problems, err.Error())
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (m Manifest) OwnsRabbitMQVHost(vhost string) bool {
	return contains(m.ManagedComponents.RabbitMQ.VHosts, vhost)
}

func (m Manifest) OwnsRabbitMQUser(user string) bool {
	return contains(m.ManagedComponents.RabbitMQ.Users, user)
}

func (m Manifest) OwnsCanalDestination(destination string) bool {
	return contains(m.ManagedComponents.Canal.Destinations, destination)
}

func Save(path string, manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func Load(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func validateMode(field, mode string) error {
	switch strings.TrimSpace(mode) {
	case ModeManaged, ModeExternal:
		return nil
	default:
		return fmt.Errorf("%s must be managed or external", field)
	}
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
