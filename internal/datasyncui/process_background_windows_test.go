//go:build windows

package datasyncui

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestConfigureBackgroundProcessDetachesFromSSHJob(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "exit", "0")
	configureBackgroundProcess(cmd)

	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow {
		t.Fatal("background process must be hidden")
	}
	want := uint32(windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS | windows.CREATE_BREAKAWAY_FROM_JOB)
	if cmd.SysProcAttr.CreationFlags&want != want {
		t.Fatalf("background process flags=%#x, want all %#x", cmd.SysProcAttr.CreationFlags, want)
	}
}
