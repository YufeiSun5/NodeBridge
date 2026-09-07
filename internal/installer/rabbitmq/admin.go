package rabbitmq

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

type CommandRunner func(context.Context, string, ...string) (string, error)

type Admin struct {
	Runner CommandRunner
	CLI    string
}

func NewAdmin() Admin {
	return Admin{Runner: runCommand}
}

func (a Admin) EnsureForNode(ctx context.Context, cfg appconfig.Config) error {
	if !appconfig.IsManagedRabbitMQ(cfg) {
		return nil
	}
	identity, err := appconfig.ManagedRabbitMQIdentityFor(cfg.Mode, cfg.Node.ID)
	if err != nil {
		return err
	}
	if cfg.Mode == appconfig.ModeServer {
		return a.ensureUser(ctx, identity.ServerVHost, identity.ServerUser, ".*", ".*", ".*")
	}
	return a.ensureUser(ctx, identity.LocalVHost, identity.LocalUser, ".*", ".*", ".*")
}

func (a Admin) EnsureServerEdgeUser(ctx context.Context, nodeID string) error {
	identity, err := appconfig.ManagedRabbitMQIdentityFor(appconfig.ModeEdge, nodeID)
	if err != nil {
		return err
	}
	return a.ensureUser(ctx, identity.ServerVHost, identity.ServerUser, "^$", `server\.ingress\..*`, regexpLiteral(nodeID)+`\.downlink\..*`)
}

func (a Admin) ensureUser(ctx context.Context, vhost, username, configure, write, read string) error {
	run, err := a.commandRunner()
	if err != nil {
		return err
	}
	cli, err := a.commandPath()
	if err != nil {
		return err
	}
	vhosts, err := run(ctx, cli, "list_vhosts", "--silent")
	if err != nil {
		return fmt.Errorf("list RabbitMQ vhosts: %w", err)
	}
	if !lineExists(vhosts, vhost) {
		if _, err := run(ctx, cli, "add_vhost", vhost); err != nil {
			return fmt.Errorf("create RabbitMQ vhost %s: %w", vhost, err)
		}
	}
	users, err := run(ctx, cli, "list_users", "--silent")
	if err != nil {
		return fmt.Errorf("list RabbitMQ users: %w", err)
	}
	if !firstColumnExists(users, username) {
		if _, err := run(ctx, cli, "add_user", username, appconfig.ManagedRabbitMQPassword); err != nil {
			return fmt.Errorf("create RabbitMQ user %s: %w", username, err)
		}
	}
	if _, err := run(ctx, cli, "change_password", username, appconfig.ManagedRabbitMQPassword); err != nil {
		return fmt.Errorf("change RabbitMQ password for %s: %w", username, err)
	}
	if _, err := run(ctx, cli, "set_permissions", "-p", vhost, username, configure, write, read); err != nil {
		return fmt.Errorf("set RabbitMQ permissions for %s: %w", username, err)
	}
	return nil
}

func (a Admin) commandRunner() (CommandRunner, error) {
	if a.Runner != nil {
		return a.Runner, nil
	}
	return nil, fmt.Errorf("RabbitMQ command runner is not configured")
}

func (a Admin) commandPath() (string, error) {
	if strings.TrimSpace(a.CLI) != "" {
		return a.CLI, nil
	}
	if path, err := exec.LookPath("rabbitmqctl.bat"); err == nil {
		return path, nil
	}
	var candidates []string
	for _, root := range []string{os.Getenv("ProgramW6432"), os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if root == "" {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(root, "RabbitMQ Server", "rabbitmq_server-*", "sbin", "rabbitmqctl.bat"))
		candidates = append(candidates, matches...)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("rabbitmqctl.bat was not found")
	}
	sort.Strings(candidates)
	return candidates[len(candidates)-1], nil
}

func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("managed RabbitMQ user changes require Windows")
	}
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	return string(output), nil
}

func lineExists(output, value string) bool {
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == value {
			return true
		}
	}
	return false
}

func firstColumnExists(output, value string) bool {
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == value {
			return true
		}
	}
	return false
}

func regexpLiteral(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `.`, `\.`, `+`, `\+`, `*`, `\*`, `?`, `\?`, `(`, `\(`, `)`, `\)`, `[`, `\[`, `]`, `\]`, `{`, `\{`, `}`, `\}`, `^`, `\^`, `$`, `\$`, `|`, `\|`)
	return replacer.Replace(value)
}
