package rules

import "testing"

func TestDownlinkTargetDatabase(t *testing.T) {
	for _, tc := range []struct{ direction, target, fallback, want string }{
		{DirectionServerToEdge, "explicit_target", "local_default", "explicit_target"},
		{DirectionServerToEdge, "", "local_default", "local_default"},
		{DirectionEdgeToServer, "central_target", "local_default", "local_default"},
		{DirectionEdgeToServer, "central_target", "", "central_target"},
		{DirectionServerToEdge, "", "", "source_db"},
	} {
		r := SyncRule{Direction: tc.direction, DatabaseName: "source_db", TargetDatabaseName: tc.target}
		if got := r.DownlinkTargetDatabase(tc.fallback); got != tc.want {
			t.Fatalf("%+v got %s", tc, got)
		}
	}
}
