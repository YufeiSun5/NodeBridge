package alignment

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

func TestSnapshotDigestFraming(t *testing.T) {
	inputs := [][][]byte{
		{nil}, {{}}, {[]byte("a"), []byte("bc")}, {[]byte("ab"), []byte("c")},
		{{0, 1, 127, 128, 255}}, {[]byte("18446744073709551615")},
	}
	seen := map[string]bool{}
	for _, values := range inputs {
		h := sha256.New()
		writeRowDigest(h, values)
		digest := fmt.Sprintf("%x", h.Sum(nil))
		if seen[digest] {
			t.Fatal("distinct rows share digest framing")
		}
		seen[digest] = true
	}
}

func TestCopyRequiresConfirmationAndDisabledRuleBeforeIO(t *testing.T) {
	r, left, right := fixture()
	left.HasRows = true
	p, err := BuildPlan(r, left, right, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CopySnapshot(context.Background(), nil, nil, p, r, false); err == nil || err.Error() != "alignment_confirmation_required" {
		t.Fatalf("confirmation: %v", err)
	}
	r.Enable = true
	p, err = BuildPlan(r, left, right, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CopySnapshot(context.Background(), nil, nil, p, r, true); err == nil || err.Error() != "alignment_requires_disabled_rule" {
		t.Fatalf("enabled: %v", err)
	}
}
