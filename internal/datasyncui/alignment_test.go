package datasyncui

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

func TestInitialAlignmentRequiresAdminAndConfirmation(t *testing.T) {
	app, _, _ := newTempApp(t)
	cfg := validConfig()
	app.config = &cfg
	request := uiapi.InitialAlignmentRequest{RuleID: "rule", PeerNodeID: "peer", Confirm: true}
	if _, err := app.StartInitialAlignment(request); err == nil {
		t.Fatal("locked start accepted")
	}
	if _, err := app.InterruptInitialAlignment(); err == nil {
		t.Fatal("locked interrupt accepted")
	}
	unlockTestAdmin(t, app)
	request.Confirm = false
	if _, err := app.StartInitialAlignment(request); err == nil {
		t.Fatal("unconfirmed start accepted")
	}
}

func TestInitialAlignmentAsyncSuccessAndSingleOperation(t *testing.T) {
	app, configPath, _ := newTempApp(t)
	unlockTestAdmin(t, app)
	entered, release, exited := make(chan struct{}), make(chan struct{}), make(chan struct{})
	app.alignment.run = func(ctx context.Context, config, rules string, request alignment.ConfiguredRequest, progress func(string)) (alignment.CutoverProof, error) {
		defer close(exited)
		if config != configPath || request.RuleID != "rule" || request.PeerID != "peer" || !request.Confirm {
			return alignment.CutoverProof{}, errors.New("wrong request")
		}
		progress("copying")
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return alignment.CutoverProof{}, ctx.Err()
		}
		return alignment.CutoverProof{Plan: alignment.Plan{ID: "proof-plan"}, Result: alignment.CopyResult{Rows: 17}}, nil
	}
	request := uiapi.InitialAlignmentRequest{RuleID: "rule", PeerNodeID: "peer", Confirm: true}
	if _, err := app.StartInitialAlignment(request); err != nil {
		t.Fatal(err)
	}
	<-entered
	if status := app.GetInitialAlignmentStatus(); !status.Running || status.Stage != "copying" {
		t.Fatal(status)
	}
	if _, err := app.StartInitialAlignment(request); err == nil {
		t.Fatal("concurrent operation accepted")
	}
	close(release)
	<-exited
	status := waitAlignmentStatus(t, app)
	if status.Stage != "completed" || status.Rows != 17 || status.PlanID != "proof-plan" {
		t.Fatal(status)
	}
	var saved uiapi.InitialAlignmentStatus
	data, err := os.ReadFile(configPath + ".alignment-status.json")
	if err != nil || json.Unmarshal(data, &saved) != nil || saved.Stage != "completed" {
		t.Fatal(string(data), err)
	}
}

func TestInitialAlignmentInterruptRedactsAndReopenIsUnknown(t *testing.T) {
	app, configPath, _ := newTempApp(t)
	unlockTestAdmin(t, app)
	app.config.MySQL.Password = "alignment-private-password"
	app.alignment.run = func(ctx context.Context, _, _ string, _ alignment.ConfiguredRequest, _ func(string)) (alignment.CutoverProof, error) {
		<-ctx.Done()
		return alignment.CutoverProof{}, errors.New("connection failed alignment-private-password")
	}
	if _, err := app.StartInitialAlignment(uiapi.InitialAlignmentRequest{RuleID: "rule", PeerNodeID: "peer", Confirm: true}); err != nil {
		t.Fatal(err)
	}
	reopened := &App{configPath: configPath}
	if status := reopened.GetInitialAlignmentStatus(); status.Running || status.Stage != "unknown" {
		t.Fatal(status)
	}
	if _, err := app.InterruptInitialAlignment(); err != nil {
		t.Fatal(err)
	}
	status := waitAlignmentStatus(t, app)
	if status.Stage != "failed" || strings.Contains(status.Message, "alignment-private-password") {
		t.Fatal(status)
	}
	data, err := os.ReadFile(configPath + ".alignment-status.json")
	if err != nil || strings.Contains(string(data), "alignment-private-password") {
		t.Fatal("persisted secret", err)
	}
}

func waitAlignmentStatus(t *testing.T, app *App) uiapi.InitialAlignmentStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := app.GetInitialAlignmentStatus()
		if !status.Running {
			return status
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("alignment task did not finish")
	return uiapi.InitialAlignmentStatus{}
}
