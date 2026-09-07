//go:build !windows

package agentstate

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
)

func lockFile(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return ErrRunning
	}
	return err
}
