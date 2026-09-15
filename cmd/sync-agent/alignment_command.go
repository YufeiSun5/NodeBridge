package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/YufeiSun5/NodeBridge/internal/agentlog"
	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

func runInitialAlignment(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("initial-alignment", flag.ContinueOnError)
	flags.SetOutput(stderr)
	config := flags.String("config", "", "saved local configuration")
	rules := flags.String("rules", "", "saved local rule file")
	rule := flags.String("rule", "", "rule ID to align")
	peer := flags.String("peer", "", "explicit peer node ID")
	confirm := flags.Bool("confirm", false, "confirm first-copy and durable isolation of superseded legacy input")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *config == "" || *rules == "" || *rule == "" || *peer == "" {
		return fmt.Errorf("config, rules, rule and peer are required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	proof, err := alignment.RunConfigured(ctx, *config, *rules, alignment.ConfiguredRequest{RuleID: *rule, PeerID: *peer, Confirm: *confirm}, func(stage string) { _, _ = fmt.Fprintf(stdout, "alignment stage=%s\n", stage) })
	if err != nil {
		message := "initial alignment failed; local configuration could not be loaded for diagnostics"
		if cfg, loadErr := appconfig.LoadFileAllowIncomplete(*config); loadErr == nil {
			message = agentlog.ForConfig(cfg)(err.Error())
		}
		_, _ = fmt.Fprintln(stderr, message)
		return err
	}
	return json.NewEncoder(stdout).Encode(struct {
		PlanID  string `json:"plan_id"`
		ProofID string `json:"proof_id"`
		Rows    int64  `json:"rows"`
	}{proof.Plan.ID, proof.ID, proof.Result.Rows})
}
