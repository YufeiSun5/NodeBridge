package alignment

import (
	"errors"
	"time"
)

// Raw and encoded limits are separate because binary values use base64 on wire.
const MaxSnapshotDataBytes = 256 * 1024 * 1024
const MaxSnapshotTransferBytes = 512 * 1024 * 1024
const SnapshotTimeout = 15 * time.Minute

type snapshotBudget struct {
	data int
	wire int
}

func (b *snapshotBudget) add(frame SnapshotFrame, wireBytes int) error {
	if wireBytes < 0 || wireBytes > MaxSnapshotTransferBytes-b.wire {
		return errors.New("alignment_transfer_limit_exceeded")
	}
	data := b.data
	for _, value := range frame.Values {
		if len(value) > MaxSnapshotDataBytes-data {
			return errors.New("alignment_data_limit_exceeded")
		}
		data += len(value)
	}
	b.data, b.wire = data, b.wire+wireBytes
	return nil
}
