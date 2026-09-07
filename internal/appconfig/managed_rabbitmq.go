package appconfig

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const ManagedRabbitMQPassword = "1234"

const (
	ManagedEdgeVHost   = "/nodebridge-edge"
	ManagedServerVHost = "/nodebridge-server"
	ManagedServerUser  = "nb-server-sync"
)

var managedNodeIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type ManagedRabbitMQIdentity struct {
	LocalUser   string
	ServerUser  string
	LocalVHost  string
	ServerVHost string
}

func ManagedRabbitMQIdentityFor(mode, nodeID string) (ManagedRabbitMQIdentity, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	nodeID = strings.ToLower(strings.TrimSpace(nodeID))
	if mode == ModeServer {
		return ManagedRabbitMQIdentity{ServerUser: ManagedServerUser, ServerVHost: ManagedServerVHost}, nil
	}
	if mode != ModeEdge {
		return ManagedRabbitMQIdentity{}, fmt.Errorf("mode must be edge or server")
	}
	if nodeID == "" || !managedNodeIDPattern.MatchString(nodeID) {
		return ManagedRabbitMQIdentity{}, fmt.Errorf("node.id must contain only letters, numbers, dot, underscore, or hyphen")
	}
	return ManagedRabbitMQIdentity{
		LocalUser:   "nb-" + nodeID + "-local",
		ServerUser:  "nb-" + nodeID,
		LocalVHost:  ManagedEdgeVHost,
		ServerVHost: ManagedServerVHost,
	}, nil
}

func IsManagedRabbitMQ(cfg Config) bool {
	return strings.EqualFold(strings.TrimSpace(cfg.RabbitMQ.Mode), "managed") || cfg.RabbitMQ.Install
}

// NormalizeManagedRabbitMQ keeps generated credentials aligned with mode and node.id.
func NormalizeManagedRabbitMQ(cfg *Config) error {
	if cfg == nil || !IsManagedRabbitMQ(*cfg) {
		return nil
	}
	identity, err := ManagedRabbitMQIdentityFor(cfg.Mode, cfg.Node.ID)
	if err != nil {
		return err
	}
	cfg.RabbitMQ.Mode = "managed"
	cfg.RabbitMQ.Install = true
	cfg.RabbitMQ.Password = ManagedRabbitMQPassword
	if cfg.Mode == ModeServer {
		cfg.RabbitMQ.Username = identity.ServerUser
		cfg.RabbitMQ.VHost = identity.ServerVHost
		cfg.RabbitMQ.ServerURL, err = managedAMQPURL(cfg.RabbitMQ.ServerURL, "127.0.0.1:5672", identity.ServerUser, identity.ServerVHost)
		return err
	}
	cfg.RabbitMQ.Username = identity.LocalUser
	cfg.RabbitMQ.VHost = identity.LocalVHost
	cfg.RabbitMQ.LocalURL, err = managedAMQPURL(cfg.RabbitMQ.LocalURL, "127.0.0.1:5672", identity.LocalUser, identity.LocalVHost)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.RabbitMQ.ServerURL) != "" {
		cfg.RabbitMQ.ServerURL, err = managedAMQPURL(cfg.RabbitMQ.ServerURL, "", identity.ServerUser, identity.ServerVHost)
	}
	return err
}

func managedAMQPURL(current, fallbackHost, username, vhost string) (string, error) {
	host := fallbackHost
	if strings.TrimSpace(current) != "" {
		parsed, err := url.Parse(current)
		if err != nil {
			return "", fmt.Errorf("parse RabbitMQ URL: %w", err)
		}
		if parsed.Host != "" {
			host = parsed.Host
		}
	}
	if host == "" {
		return "", nil
	}
	value := &url.URL{
		Scheme:  "amqp",
		User:    url.UserPassword(username, ManagedRabbitMQPassword),
		Host:    host,
		Path:    "/" + vhost,
		RawPath: "/" + url.PathEscape(vhost),
	}
	return value.String(), nil
}
