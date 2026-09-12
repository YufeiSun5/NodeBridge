package main

import "testing"

func TestRevisionTrackerRejectsRollback(t *testing.T) {
	r := revisionTracker{}
	for _, v := range []int64{1, 2, 2, 9007199254740993} {
		if err := r.observe("edge/1", v); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.observe("edge/1", 9007199254740992); err == nil {
		t.Fatal("rollback accepted")
	}
	if err := r.observe("server/1", 1); err != nil {
		t.Fatal("independent key rejected")
	}
}

func TestCanalBootstrapContainsExactNumericBoundaries(t *testing.T) {
	for _, l := range layouts("test") {
		r := makeRow(settings{}, l, l.Offset+1000, 1)
		if r[12] != int64(9007199254740993) || r[13] != int64(9223372036854775807) || r[14] != int64(-9223372036854775808) || r[24] != "999999999999.123456" {
			t.Fatalf("missing boundary: %v", r[12:26])
		}
	}
}
