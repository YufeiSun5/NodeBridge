package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("sync-agent", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to sync-agent config file")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}

	fmt.Fprintf(stdout, "sync-agent ready mode=%s node_id=%s node_name=%s\n", cfg.Mode, cfg.Node.ID, cfg.Node.Name)
	return nil
}
