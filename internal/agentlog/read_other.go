//go:build !windows

package agentlog

import "os"

func openLogRead(path string) (*os.File, error) { return os.Open(path) }
