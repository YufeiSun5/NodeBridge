package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestReadPairManifestStrictBounded(t *testing.T) {
	good := pairManifest{Version: 1, Pairs: []rulecheck.ObservedPair{{EdgeNode: "edge", ServerNode: "server"}}}
	body, err := json.Marshal(good)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", string(body), true},
		{"unknown", `{"version":1,"pairs":[{}],"authorized":true}`, false},
		{"nested unknown", `{"version":1,"pairs":[{"credential":"not allowed"}]}`, false},
		{"version", `{"version":2,"pairs":[{}]}`, false},
		{"empty", `{"version":1,"pairs":[]}`, false},
		{"trailing", string(body) + `{}`, false},
		{"malformed", `{`, false},
		{"too large", strings.Repeat(" ", 16<<20+1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pairs.json")
			if err := os.WriteFile(path, []byte(test.body), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := readPairManifest(path)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v err=%v", test.valid, err)
			}
			if test.valid && !reflect.DeepEqual(got, good.Pairs) {
				t.Fatal("round trip mismatch")
			}
		})
	}
	if _, err := readPairManifest(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("accepted missing file")
	}
}

func TestResolveOneWayNeverReadsManifestOrMySQL(t *testing.T) {
	cfg := &appconfig.Config{Mode: appconfig.ModeEdge, Node: appconfig.NodeConfig{ID: "edge"}}
	set := &rules.RuleSet{Rules: []rules.SyncRule{{ID: "one", Enable: true, Direction: rules.DirectionEdgeToServer, DatabaseName: "db", TableName: "rows", PrimaryKeys: []string{"id"}}}}
	got, err := resolveEndpointRules(context.Background(), cfg, set, "missing-manifest")
	if err != nil || !reflect.DeepEqual(got.Capture, *set) || !reflect.DeepEqual(got.Incoming, *set) {
		t.Fatalf("%+v %v", got, err)
	}
	set.Rules[0].Direction, set.Rules[0].ConflictPolicy = rules.DirectionBidirectional, rules.ConflictLastWriteWin
	if _, err := resolveEndpointRules(context.Background(), cfg, set, ""); err == nil {
		t.Fatal("accepted missing paired endpoints")
	}
}

func TestActivePairGroupsPreserveRemoteMappings(t *testing.T) {
	local := rulecheck.ObservedPair{EdgeNode: "edge-1", ServerNode: "server", Server: rulecheck.Schema{Database: "center", Table: "rows"}, Rule: rules.SyncRule{ID: "local"}}
	remote := local
	remote.EdgeNode, remote.Rule.ID = "edge-2", "remote"
	unrelated := local
	unrelated.Rule.ID, unrelated.Server.Table = "disabled", "other"
	foreign := local
	foreign.EdgeNode, foreign.ServerNode = "foreign-edge", "foreign-server"
	pairs := []rulecheck.ObservedPair{remote, unrelated, local, foreign}
	before, _ := json.Marshal(pairs)
	for _, node := range []string{"edge-1", "server"} {
		mode := "edge"
		if node == "server" {
			mode = "server"
		}
		for _, direction := range []string{rules.DirectionBidirectional, rules.DirectionEdgeToServer} {
			for _, enabled := range []bool{false, true} {
				set := rules.RuleSet{Rules: []rules.SyncRule{{ID: "local", Enable: enabled, Direction: direction}, {ID: "disabled", Direction: rules.DirectionBidirectional}}}
				got := activePairGroups(node, mode, set, pairs)
				if enabled && direction == rules.DirectionBidirectional {
					if !reflect.DeepEqual(got, []rulecheck.ObservedPair{remote, local}) {
						t.Fatal("lost remote mapping or included unrelated group", node, got)
					}
				} else if len(got) != 0 {
					t.Fatal("inactive group selected", node, got)
				}
			}
		}
	}
	after, _ := json.Marshal(pairs)
	if string(before) != string(after) {
		t.Fatal("manifest mutated")
	}
}
