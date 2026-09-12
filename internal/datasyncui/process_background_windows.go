//go:build windows

package datasyncui

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func configureBackgroundProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP |
			windows.DETACHED_PROCESS |
			windows.CREATE_BREAKAWAY_FROM_JOB,
	}
}
