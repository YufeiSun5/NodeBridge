package conflict

import (
	"strings"
	"testing"
	"time"
)

func version(node, id string, micros int64, deleted bool) Version {
	return Version{Time: time.UnixMicro(micros).UTC(), TimeSource: SourceBinlog, OriginNodeID: node, EventID: id, PayloadHash: strings.Repeat("a", 64), Deleted: deleted, BinlogFile: "mysql-bin.000001", BinlogPos: 4}
}

func TestLastSourceTimeWins(t *testing.T) {
	old := version("edge", "old", 1000000, false)
	newer := version("server", "new", 2000000, false)
	for _, tc := range []struct {
		name     string
		current  *Version
		incoming Version
		want     string
	}{
		{"first", nil, old, Apply},
		{"later source time", &old, newer, Apply},
		{"late delivery of older event", &newer, old, Superseded},
		{"duplicate", &newer, newer, Duplicate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(tc.current, tc.incoming)
			if err != nil || got != tc.want {
				t.Fatalf("got %s, %v; want %s", got, err, tc.want)
			}
		})
	}
}

func TestTombstonePreventsOldEventResurrection(t *testing.T) {
	deleted := version("edge", "delete", 2000000, true)
	for _, old := range []Version{version("edge", "insert", 1000000, false), version("server", "update", 1999999, false)} {
		if got, err := Resolve(&deleted, old); err != nil || got != Superseded {
			t.Fatalf("%s %v", got, err)
		}
	}
	if got, err := Resolve(&deleted, version("server", "recreate", 2000001, false)); err != nil || got != Apply {
		t.Fatalf("%s %v", got, err)
	}
}

func TestEqualTimeOrderConverges(t *testing.T) {
	events := []Version{version("a", "1", 1000000, false), version("b", "1", 1000000, true), version("b", "2", 1000000, false)}
	for _, order := range [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		var current *Version
		for _, i := range order {
			next := events[i]
			got, err := Resolve(current, next)
			if err != nil {
				t.Fatal(err)
			}
			if got == Apply {
				current = &next
			}
		}
		if current.OriginNodeID != "b" || current.EventID != "2" {
			t.Fatalf("order %v diverged: %+v", order, current)
		}
	}
}

func TestRejectInvalidOrChangedVersion(t *testing.T) {
	base := version("a", "1", 1000000, false)
	for _, mutate := range []func(*Version){
		func(v *Version) { v.Time = time.Time{} },
		func(v *Version) { v.Time = time.Unix(0, 0) },
		func(v *Version) { v.Time = v.Time.Add(time.Nanosecond) },
		func(v *Version) { v.TimeSource = "received_at" },
		func(v *Version) { v.OriginNodeID = "" },
		func(v *Version) { v.PayloadHash = strings.Repeat("g", 64) },
		func(v *Version) { v.TimeSource = SourceCommand },
		func(v *Version) { v.Time = v.Time.Add(time.Second) },
		func(v *Version) { v.Deleted = true },
		func(v *Version) { v.PayloadHash = strings.Repeat("b", 64) },
	} {
		changed := base
		mutate(&changed)
		if _, err := Resolve(&base, changed); err == nil {
			t.Fatalf("accepted changed/invalid version: %+v", changed)
		}
	}
}

func TestTimeZonesUseInstantNotWallClock(t *testing.T) {
	a := version("a", "1", 1000000, false)
	b := a
	b.Time = a.Time.In(time.FixedZone("UTC+8", 8*3600))
	if got, err := Resolve(&a, b); err != nil || got != Duplicate {
		t.Fatalf("%s %v", got, err)
	}
}
