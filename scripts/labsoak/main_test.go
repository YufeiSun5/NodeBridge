package main

import "testing"

func TestExpectedRows(t *testing.T) {
	for _, tc := range []struct{ batches, rows int }{{0, 100}, {1, 120}, {2, 140}, {3, 150}, {5040, 50520}} {
		if got := expectedRows(tc.batches); got != tc.rows {
			t.Fatalf("batches=%d got=%d want=%d", tc.batches, got, tc.rows)
		}
	}
}
