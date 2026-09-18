package datasyncui

import "testing"

func TestInitializeSystemDatabaseRequiresAdmin(t *testing.T) {
	a := &App{}
	result := a.InitializeSystemDatabase()
	if result.OK || result.Status != "locked" {
		t.Fatalf("unlocked initialization: %+v", result)
	}
}
