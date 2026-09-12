package alignment

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSnapshotFramePreservesRawValues(t *testing.T) {
	f := SnapshotFrame{PlanID: strings.Repeat("a", 64), Sequence: 12, Values: [][]byte{nil, {}, {0, 128, 255}, []byte("18446744073709551615"), []byte("12345678901234567890.1234567890")}}
	b, err := EncodeSnapshotFrame(f)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeSnapshotFrame(b)
	if err != nil || !reflect.DeepEqual(got, f) {
		t.Fatalf("frame changed: %#v %v", got, err)
	}
	end := CopyResult{PlanID: f.PlanID, Rows: 13, Digest: strings.Repeat("b", 64)}
	f = SnapshotFrame{PlanID: f.PlanID, Sequence: 13, End: &end}
	b, err = EncodeSnapshotFrame(f)
	if err != nil {
		t.Fatal(err)
	}
	got, err = DecodeSnapshotFrame(b)
	if err != nil || !reflect.DeepEqual(got, f) {
		t.Fatal("end changed", err)
	}
}

func TestSnapshotFrameRejectsMalformedAndOversized(t *testing.T) {
	valid := SnapshotFrame{PlanID: strings.Repeat("a", 64), Values: [][]byte{[]byte("1")}}
	b, err := EncodeSnapshotFrame(valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{append(append([]byte{}, b...), []byte("{}")...), append(append([]byte{}, b...), 'x'), b[:len(b)-1], []byte(`{"unknown":true}`), bytes.Repeat([]byte("x"), MaxSnapshotFrameBytes+1)} {
		if _, err := DecodeSnapshotFrame(raw); err == nil {
			t.Fatal("invalid wire frame accepted")
		}
	}
	for _, change := range []func(*SnapshotFrame){
		func(f *SnapshotFrame) { f.PlanID = "bad" },
		func(f *SnapshotFrame) { f.Sequence = -1 },
		func(f *SnapshotFrame) { f.Values = nil },
		func(f *SnapshotFrame) { f.Values = [][]byte{make([]byte, MaxSnapshotFrameBytes)} },
		func(f *SnapshotFrame) { f.Values = [][]byte{make([]byte, MaxSnapshotFrameBytes*3/4)} },
		func(f *SnapshotFrame) { f.End = &CopyResult{PlanID: f.PlanID, Digest: strings.Repeat("b", 64)} },
		func(f *SnapshotFrame) {
			f.Values = nil
			f.End = &CopyResult{PlanID: f.PlanID, Rows: 1, Digest: strings.Repeat("b", 64)}
		},
		func(f *SnapshotFrame) {
			f.Values = nil
			f.End = &CopyResult{PlanID: f.PlanID, Digest: strings.Repeat("z", 64)}
		},
	} {
		f := valid
		change(&f)
		if _, err := EncodeSnapshotFrame(f); err == nil {
			t.Fatal("invalid frame encoded")
		}
		raw, _ := json.Marshal(f)
		if _, err := DecodeSnapshotFrame(raw); err == nil {
			t.Fatal("invalid frame decoded")
		}
	}
}

func TestStreamRequiresConfirmationBeforeIO(t *testing.T) {
	r, left, right := fixture()
	left.HasRows = true
	p, err := BuildPlan(r, left, right, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExportSnapshot(context.Background(), nil, p, r, false, nil); err == nil || err.Error() != "alignment_confirmation_required" {
		t.Fatal(err)
	}
	if _, err := NewSnapshotReceiver(context.Background(), nil, p, r, false); err == nil || err.Error() != "alignment_confirmation_required" {
		t.Fatal(err)
	}
}
