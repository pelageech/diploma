package timeutil

import "time"

// Ticker is the interface that holds a channel that delivers `tick` events.
// The purpose of this interface is an easy way to isolate logic from
// time-specific implementations.
type Ticker interface {
	Stop()
	Reset(d time.Duration)
	C() <-chan time.Time
}

// NewTicker creates a new Ticker containing a channel that will sent time with
// a period specified by the duration argument.
// It adjusts the intervals or drops ticks to make up for slow receivers.
// The duration d must be greater than zero; if not, NewTicker will panic.
// Stop the ticker to release associated resources.
//
// It uses time package as Ticker implementation.
func NewTicker(d time.Duration) Ticker {
	return timeTicker{time.NewTicker(d)}
}

type timeTicker struct {
	t *time.Ticker
}

func (t timeTicker) Reset(d time.Duration) {
	t.t.Reset(d)
}

func (t timeTicker) C() <-chan time.Time {
	return t.t.C
}

func (t timeTicker) Stop() {
	t.t.Stop()
}
