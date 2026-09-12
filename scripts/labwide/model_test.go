package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func fixture(t *testing.T) schema {
	t.Helper()
	b, err := os.ReadFile("../fixtures/longtest-wide50.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var s schema
	if err = json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestWideRowsAndCoverage(t *testing.T) {
	s := fixture(t)
	if len(s.Columns) != 50 {
		t.Fatal(len(s.Columns))
	}
	c := settings{RunID: "unit", Created: "2026-09-09 00:00:00.000"}
	var coverage [50]int
	for op := int64(0); op < 2000; op++ {
		row := makeRow(c, layouts("nb_unit")[0], op+1, op+1)
		if len(row) != 50 {
			t.Fatal(len(row))
		}
		for i, v := range row {
			if !s.Columns[i].Nullable && v == nil {
				t.Fatalf("null %s", s.Columns[i].Name)
			}
		}
		for _, i := range updateIndices(op) {
			coverage[i]++
		}
	}
	for i := 12; i < 50; i++ {
		if coverage[i] == 0 {
			t.Fatalf("uncovered %s", s.Columns[i].Name)
		}
	}
}
func TestCanonicalDistinguishesNullEmptyZeroAndBoundaries(t *testing.T) {
	s := fixture(t)
	base := rawRow(makeRow(settings{}, layouts("nb_unit")[0], 1, 1))
	seen := map[string]bool{}
	for _, v := range [][]byte{nil, {}, []byte("0"), []byte("ab#cd"), []byte("9007199254740993")} {
		base[12] = v
		d := digest(s, [][][]byte{base})
		if seen[d] {
			t.Fatal("collision")
		}
		seen[d] = true
	}
	a := rawRow(makeRow(settings{}, layouts("nb_unit")[0], 1, 1))
	b := rawRow(makeRow(settings{}, layouts("nb_unit")[0], 1, 1))
	if err := compareRows(s, a, b); err != nil {
		t.Fatal(err)
	}
	b[49] = []byte("changed")
	if compareRows(s, a, b) == nil {
		t.Fatal("missed column 50")
	}
	if bytes.Equal(a[49], b[49]) {
		t.Fatal("test alias")
	}
}
func TestSevenHourRates(t *testing.T) {
	var inserts, updates int
	for second := 0; second < 420*60; second++ {
		r := rateAt(float64(second) / 60)
		inserts += r.I
		updates += r.U
	}
	if inserts != 900000 || updates != 558000 {
		t.Fatalf("%d/%d", inserts, updates)
	}
}

func TestScaledSmokeRateTotals(t *testing.T) {
	for _, test := range []struct{ duration, inserts, updates int }{{600, 21420, 13290}, {1800, 64220, 39850}, {3600, 128520, 79680}, {36000, 1285720, 797120}} {
		var inserts, updates int
		for tick := 0; tick < test.duration*390/420; tick++ {
			r := rateAt(float64(tick) * 420 / float64(test.duration))
			inserts += r.I
			updates += r.U
		}
		if inserts != test.inserts || updates != test.updates {
			t.Fatalf("duration=%d got=%d/%d want=%d/%d", test.duration, inserts, updates, test.inserts, test.updates)
		}
	}
}

func TestNullRoundTripUsesPerKeyRevision(t *testing.T) {
	for _, index := range []int{12, 24, 36, 49} {
		for round := int64(0); round < 6; round++ {
			value := updatedValue(16, round+2, index, 15+round*1000)
			if (value == nil) != (round%2 == 0) {
				t.Fatalf("column %d round %d: %v", index, round, value)
			}
		}
	}
}
func TestMappingsAreExplicit(t *testing.T) {
	l := layouts("nb_unit")
	if l[0].Stream.col("id") == l[1].Target.col("id") {
		t.Fatal("missing PK mapping")
	}
	if l[0].State.col("t01") == l[1].State.col("t01") {
		t.Fatal("missing state mapping")
	}
	for _, side := range l {
		for _, table := range []table{side.Stream, side.State, side.Target} {
			if !identifier.MatchString(table.Name) {
				t.Fatal(table.Name)
			}
		}
	}
}
