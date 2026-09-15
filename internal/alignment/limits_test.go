package alignment

import "testing"

func TestSnapshotBudgetSupports200MiBAndBoundsBothRepresentations(t *testing.T) {
	var budget snapshotBudget
	frame := SnapshotFrame{Values: [][]byte{make([]byte, 512*1024)}}
	for i := 0; i < 400; i++ {
		if err := budget.add(frame, 700*1024); err != nil {
			t.Fatalf("200 MiB payload rejected at frame %d: %v", i, err)
		}
	}
	if budget.data != 200*1024*1024 {
		t.Fatal("raw byte accounting changed")
	}
	budget.data = MaxSnapshotDataBytes
	before := budget
	if err := budget.add(frame, 1); err == nil || budget != before {
		t.Fatal("raw overflow must fail without changing the budget")
	}
	budget.data, budget.wire = 0, MaxSnapshotTransferBytes
	if err := budget.add(SnapshotFrame{}, 1); err == nil {
		t.Fatal("encoded overflow accepted")
	}
	if err := budget.add(SnapshotFrame{}, -1); err == nil {
		t.Fatal("negative size accepted")
	}
}
