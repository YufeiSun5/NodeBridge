//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	path := os.Getenv("NODEBRIDGE_INSTALLER_TEST_TRACE")
	if path == "" {
		os.Exit(2)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		os.Exit(3)
	}
	if err := json.NewEncoder(f).Encode(os.Args[1:]); err != nil {
		os.Exit(4)
	}
	if err := f.Close(); err != nil {
		os.Exit(5)
	}
	if len(os.Args) > 1 && os.Args[1] == "upgrade-system" {
		if os.Getenv("NODEBRIDGE_INSTALLER_TEST_UPGRADE_FAIL") == "1" {
			os.Exit(9)
		}
		if os.Getenv("NODEBRIDGE_INSTALLER_TEST_UPGRADE_PENDING") == "1" {
			fmt.Println(`{"status":"not_configured"}`)
			return
		}
		fmt.Println(`{"status":"upgraded"}`)
		return
	}
	fmt.Println(`{"ok":true}`)
}
