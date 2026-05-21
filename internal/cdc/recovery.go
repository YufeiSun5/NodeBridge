package cdc

import (
	"errors"
	"time"
)

var ErrFatalRecovery = errors.New("fatal cdc recovery error")

type RecoveryPolicy struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	MaxAttempts  int
}

func DefaultRecoveryPolicy() RecoveryPolicy {
	return RecoveryPolicy{
		InitialDelay: 2 * time.Second,
		MaxDelay:     30 * time.Second,
		MaxAttempts:  0,
	}
}

func (p RecoveryPolicy) NextDelay(attempt int) (time.Duration, bool) {
	if p.MaxAttempts > 0 && attempt >= p.MaxAttempts {
		return 0, false
	}
	initial := p.InitialDelay
	if initial <= 0 {
		initial = time.Second
	}
	maxDelay := p.MaxDelay
	if maxDelay <= 0 {
		maxDelay = initial
	}
	delay := initial
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay >= maxDelay {
			return maxDelay, true
		}
	}
	return delay, true
}

func Recoverable(err error) bool {
	return err != nil && !errors.Is(err, ErrFatalRecovery)
}
