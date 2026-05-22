package rabbitmq_test

import (
	"testing"

	installer "github.com/YufeiSun5/NodeBridge/internal/installer/rabbitmq"
)

func TestBuildPlanFreshInstall(t *testing.T) {
	plan := installer.BuildPlan(installer.CurrentState{}, installer.DefaultDesiredState())

	requireStep(t, plan, installer.ComponentErlang, installer.ActionInstall)
	requireStep(t, plan, installer.ComponentRabbitMQ, installer.ActionInstall)
	requireStep(t, plan, installer.ComponentService, installer.ActionInstall)
	requireStep(t, plan, installer.ComponentService, installer.ActionStart)
	requireStep(t, plan, installer.ComponentRabbitMQ, installer.ActionCreate)
	requireStep(t, plan, installer.ComponentRabbitMQ, installer.ActionGrant)
	requireStep(t, plan, installer.ComponentTopology, installer.ActionInitialize)
}

func TestBuildPlanPartialInstall(t *testing.T) {
	desired := installer.DefaultDesiredState()
	current := installer.CurrentState{
		ErlangInstalled:   true,
		RabbitMQInstalled: true,
		ServiceInstalled:  true,
		ServiceRunning:    false,
		VHosts:            map[string]bool{"/nodebridge-edge": true},
		Users:             map[string]bool{"nb-server-sync": true},
		Permissions:       map[string]bool{"nb-server-sync@/nodebridge-server": true},
	}

	plan := installer.BuildPlan(current, desired)

	requireNoStep(t, plan, installer.ComponentErlang, installer.ActionInstall)
	requireNoStep(t, plan, installer.ComponentRabbitMQ, installer.ActionInstall)
	requireStep(t, plan, installer.ComponentService, installer.ActionStart)
	requireTarget(t, plan, installer.ComponentRabbitMQ, installer.ActionCreate, "vhost:/nodebridge-server")
	requireStep(t, plan, installer.ComponentTopology, installer.ActionInitialize)
}

func TestBuildPlanExternalDoesNothing(t *testing.T) {
	desired := installer.DefaultDesiredState()
	desired.Mode = "external"

	plan := installer.BuildPlan(installer.CurrentState{}, desired)

	if len(plan.Steps) != 1 || plan.Steps[0].Target != "external-rabbitmq" {
		t.Fatalf("external rabbitmq must not be modified, got %+v", plan.Steps)
	}
}

func TestBuildPlanAlreadyReady(t *testing.T) {
	desired := installer.DefaultDesiredState()
	current := installer.CurrentState{
		ErlangInstalled:   true,
		RabbitMQInstalled: true,
		ServiceInstalled:  true,
		ServiceRunning:    true,
		VHosts:            map[string]bool{"/nodebridge-edge": true, "/nodebridge-server": true},
		Users: map[string]bool{
			"nb-server-sync":    true,
			"nb-edge-001":       true,
			"nb-edge-001-local": true,
		},
		Permissions: map[string]bool{
			"nb-server-sync@/nodebridge-server":  true,
			"nb-edge-001@/nodebridge-server":     true,
			"nb-edge-001-local@/nodebridge-edge": true,
		},
		TopologyReady: true,
	}

	plan := installer.BuildPlan(current, desired)

	requireNoStep(t, plan, installer.ComponentTopology, installer.ActionInitialize)
	requireStep(t, plan, installer.ComponentRabbitMQ, installer.ActionPreserveData)
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

func requireTarget(t *testing.T, plan installer.Plan, component, action, target string) {
	t.Helper()
	for _, step := range plan.Steps {
		if step.Component == component && step.Action == action && step.Target == target {
			return
		}
	}
	t.Fatalf("missing step component=%s action=%s target=%s in %+v", component, action, target, plan.Steps)
}
