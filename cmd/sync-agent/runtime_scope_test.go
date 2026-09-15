package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func ownedTableScope(database, table string) string {
	data, _ := json.Marshal([]string{database, table})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func TestRuntimeTableScopesUseEnabledPhysicalEndpoints(t *testing.T) {
	ep := rulecheck.EndpointRules{Capture: rules.RuleSet{Rules: []rules.SyncRule{{Enable: true, Direction: rules.DirectionEdgeToServer, DatabaseName: "local", TableName: "notes"}, {Enable: false, DatabaseName: "local", TableName: "standards"}}}, Incoming: rules.RuleSet{Rules: []rules.SyncRule{{Enable: true, Direction: rules.DirectionEdgeToServer, DatabaseName: "remote", TableName: "source", TargetDatabaseName: "local", TargetTableName: "mapped"}}}}
	scopes := runtimeTableScopes(ep)
	if len(scopes) != 2 || scopes[0].Database != "local" || scopes[0].Table != "notes" || scopes[1].Database != "local" || scopes[1].Table != "mapped" {
		t.Fatal(scopes)
	}
}
