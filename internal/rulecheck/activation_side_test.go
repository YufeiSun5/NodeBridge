package rulecheck

import (
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestActivationSideUsesLocalEndpointForBidirectional(t *testing.T) {
	for _, tc := range []struct{ mode, direction, want string }{
		{"server", rules.DirectionBidirectional, "target"},
		{"edge", rules.DirectionBidirectional, "source"},
		{"server", rules.DirectionEdgeToServer, "target"},
		{"edge", rules.DirectionEdgeToServer, "source"},
		{"server", rules.DirectionServerToEdge, "source"},
		{"edge", rules.DirectionServerToEdge, "target"},
	} {
		if got := activationSide(tc.mode, tc.direction); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}
