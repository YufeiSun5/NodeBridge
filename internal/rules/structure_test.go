package rules_test

import (
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"testing"
)

func TestStructureValidationDoesNotAuthorizeUnsupportedRuntime(t *testing.T) {
	r := rules.SyncRule{ID: "bidi", DatabaseName: "db", TableName: "rows", PrimaryKeys: []string{"id"}, Enable: true, Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin, DeleteMode: rules.DeleteHard}
	s := rules.RuleSet{Rules: []rules.SyncRule{r}}
	if err := s.ValidateStructure(); err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(); err == nil {
		t.Fatal("public runtime gate bypassed")
	}
	s.Rules[0].TableName = "invalid.name"
	if err := s.ValidateStructure(); err == nil {
		t.Fatal("structural error accepted")
	}
}
