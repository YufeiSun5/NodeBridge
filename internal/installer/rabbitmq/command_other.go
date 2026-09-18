//go:build !windows

package rabbitmq

import (
	"context"
	"fmt"
	"os/exec"
)

func adminCommand(context.Context, string, ...string) (*exec.Cmd, error) {
	return nil, fmt.Errorf("managed RabbitMQ user changes require Windows")
}
