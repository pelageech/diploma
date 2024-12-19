package timeutil

import "time"

// Now returns result of time.Now() if no TestHookTimeNow set up.
func Now() time.Time {
	if f := testHookTimeNow; f != nil {
		return f()
	}
	return time.Now()
}

// Since returns the time elapsed since t.
// It is shorthand for time.Now().Sub(t).
func Since(t time.Time) time.Duration {
	return Now().Sub(t)
}

// Until returns the duration until t.
// It is shorthand for t.Sub(time.Now()).
func Until(t time.Time) time.Duration {
	return t.Sub(Now())
}

// Timer is the interface used by node watcher to be periodically triggered to
// prepare some action.
type Timer interface {
	Reset(time.Duration) bool
	Stop() bool
	C() <-chan time.Time
}

// NewTimer creates a new Timer that will send the current time on its channel
// after at least duration d.
//
// It uses time package as timer implementation.
func NewTimer(d time.Duration) Timer {
	return timeTimer{time.NewTimer(d)}
}

type timeTimer struct {
	t *time.Timer
}

func (t timeTimer) C() <-chan time.Time {
	return t.t.C
}

func (t timeTimer) Reset(d time.Duration) bool {
	return t.t.Reset(d)
}

func (t timeTimer) Stop() bool {
	return t.t.Stop()
}

// AfterFunc waits for the duration to elapse and then calls f
// in its own goroutine. It returns a Timer that can
// be used to cancel the call using its Stop method.
func AfterFunc(d time.Duration, f func()) Timer {
	return timeTimer{time.AfterFunc(d, f)}
}
