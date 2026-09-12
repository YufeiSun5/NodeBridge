package alignment

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSnapshotReplyPhasesAndStrictFraming(t *testing.T) {
	for _, reply := range []snapshotReply{
		{PlanID: strings.Repeat("a", 64), Sequence: 0},
		{PlanID: strings.Repeat("a", 64), Sequence: 1, Error: "alignment_receive_failed"},
		{PlanID: strings.Repeat("a", 64), Sequence: 2, Result: &CopyResult{PlanID: strings.Repeat("a", 64), Rows: 2, Digest: strings.Repeat("b", 64)}},
	} {
		body, err := json.Marshal(reply)
		if err != nil {
			t.Fatal(err)
		}
		got, err := decodeSnapshotReply(body)
		if err != nil || got.PlanID != reply.PlanID || got.Sequence != reply.Sequence || got.Error != reply.Error || (got.Result == nil) != (reply.Result == nil) {
			t.Fatal("reply changed", got, err)
		}
		if _, err := decodeSnapshotReply(append(body, []byte("{}")...)); err == nil {
			t.Fatal("trailing document accepted")
		}
	}
	for _, body := range []string{
		`{"plan_id":"bad","sequence":0}`,
		`{"plan_id":"` + strings.Repeat("a", 64) + `","sequence":-1}`,
		`{"plan_id":"` + strings.Repeat("a", 64) + `","sequence":0,"extra":true}`,
		`{"plan_id":"` + strings.Repeat("a", 64) + `","sequence":0,"error":"arbitrary remote error"}`,
		strings.Repeat("x", 1025),
	} {
		if _, err := decodeSnapshotReply([]byte(body)); err == nil {
			t.Fatal("invalid reply accepted")
		}
	}
}
