package main

import "testing"

func TestLatencyPhaseUsesFixedWorkloadBoundaries(t *testing.T) {
	for _, duration := range []int{600, 25200, 36000} {
		for _, tc := range []struct {
			minute float64
			want   string
		}{{0, "steady"}, {269.9, "steady"}, {270.1, "peak"}, {299.9, "peak"}, {300.1, "update_heavy"}, {329.9, "update_heavy"}, {330.1, "steady"}} {
			if got := latencyPhase(tc.minute*float64(duration)/420, duration); got != tc.want {
				t.Fatalf("duration=%d minute=%v: %s != %s", duration, tc.minute, got, tc.want)
			}
		}
	}
}
