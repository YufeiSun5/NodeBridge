package main

import "testing"

func TestReplayProofEnforcesMetadata(t *testing.T) {
	l := layouts("test")
	want := rawRow(makeRow(settings{}, l[0], 1, 1))
	got := append([][]byte(nil), want...)
	got[6] = []byte("E1")
	proof := replayProof{"edge-001", "scada_center", l[1].State.Name, `{"setting_id":1}`, "UPDATE"}
	proofs := map[string]replayProof{"E1": proof}
	clean, err := checkReplay(want, got, "server-001", l[1].State, "scada_center", proofs)
	if err != nil || len(clean[6]) != 0 || string(got[6]) != "E1" {
		t.Fatalf("valid proof: %v", err)
	}
	if _, err = checkReplay(want, got, "edge-001", l[0].State, "scada_edge", proofs); err == nil {
		t.Fatal("source replay accepted")
	}
	for _, field := range []string{"origin", "database", "table", "pk", "operation", "missing"} {
		t.Run(field, func(t *testing.T) {
			p := proof
			ps := map[string]replayProof{}
			switch field {
			case "origin":
				p.Origin = "wrong"
			case "database":
				p.Database = "wrong"
			case "table":
				p.Table = "wrong"
			case "pk":
				p.PK = `{"id":1}`
			case "operation":
				p.Operation = "DDL"
			}
			if field != "missing" {
				ps["E1"] = p
			}
			if _, e := checkReplay(want, got, "server-001", l[1].State, "scada_center", ps); e == nil {
				t.Fatal("bad proof accepted")
			}
		})
	}
}
