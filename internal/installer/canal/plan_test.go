package canal_test

import (
	"testing"

	installer "github.com/YufeiSun5/NodeBridge/internal/installer/canal"
)

func TestBuildPlanFreshInstall(t *testing.T) {
	plan := installer.BuildPlan(installer.CurrentState{}, installer.DefaultDesiredState("edge-001"))

	requireStep(t, plan, installer.ComponentCanal, installer.ActionInstall)
	requireStep(t, plan, installer.ComponentCanalConfig, installer.ActionCreate)
	requireStep(t, plan, installer.ComponentCanalConfig, installer.ActionInitialize)
	requireStep(t, plan, installer.ComponentService, installer.ActionStart)
}

func TestBuildPlanExternalDoesNothing(t *testing.T) {
	desired := installer.DefaultDesiredState("edge-001")
	desired.Mode = "external"

	plan := installer.BuildPlan(installer.CurrentState{}, desired)

	if len(plan.Steps) != 1 || plan.Steps[0].Target != "external-canal" {
		t.Fatalf("external canal must not be modified, got %+v", plan.Steps)
	}
}

func TestBuildPlanAlreadyReady(t *testing.T) {
	desired := installer.DefaultDesiredState("edge-001")
	current := installer.CurrentState{
		CanalInstalled: true,
		ServiceRunning: true,
		ConfigDirs:     map[string]bool{desired.ConfigDir: true},
		Destinations:   map[string]bool{"nodebridge-edge-001": true},
	}

	plan := installer.BuildPlan(current, desired)

	requireNoStep(t, plan, installer.ComponentCanalConfig, installer.ActionInitialize)
	requireStep(t, plan, installer.ComponentCanal, installer.ActionPreserve)
}

func requireStep(t *testing.T, plan installer.Plan, component, action string) {
	t.Helper()
	for _, step := range plan.Steps {
		if step.Component == component && step.Action == action {
			return
		}
	}
	t.Fatalf("missing step component=%s action=%s in %+v", component, action, plan.Steps)
}

func requireNoStep(t *testing.T, plan installer.Plan, component, action string) {
	t.Helper()
	for _, step := range plan.Steps {
		if step.Component == component && step.Action == action {
			t.Fatalf("unexpected step component=%s action=%s in %+v", component, action, plan.Steps)
		}
	}
}
