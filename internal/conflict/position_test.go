package conflict

import "testing"

func TestSameSecondUsesSourceBinlogOrderNotRandomEventID(t *testing.T) {
	a := version("edge", "z-earlier-id", 1000000, false)
	b := version("edge", "a-later-id", 1000000, true)
	b.BinlogPos = 100
	if got, err := Resolve(&a, b); err != nil || got != Apply {
		t.Fatalf("%s %v", got, err)
	}
	if got, err := Resolve(&b, a); err != nil || got != Superseded {
		t.Fatalf("%s %v", got, err)
	}
	a.BinlogFile = "mysql-bin.9"
	a.BinlogPos = 999
	b.BinlogFile = "mysql-bin.10"
	b.BinlogPos = 4
	if got, err := Resolve(&a, b); err != nil || got != Apply {
		t.Fatalf("rotation order: %s %v", got, err)
	}
}
