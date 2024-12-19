package timetest

import (
	"sync"
	"time"

	"github.com/pelageech/diploma/schedule/timeutil"
)

// NewTimer creates fake timer that fires when ch has value.
func NewTimer(ch chan time.Time) timeutil.Timer {
	return &FakeTimer{
		Ch: ch,
	}
}

type FakeTimer struct {
	// Ch is intended to be send-only for users.
	Ch chan time.Time
	// StopCh is intended to be receive-only for users.
	StopCh chan struct{}
	// ResetCh is intended to be receive-only for users.
	ResetCh chan time.Duration
}

func (f *FakeTimer) C() <-chan time.Time {
	return f.Ch
}

func (f *FakeTimer) Reset(d time.Duration) bool {
	if f.ResetCh != nil {
		f.ResetCh <- d
	}
	return true
}

func (f *FakeTimer) Stop() bool {
	if f.StopCh != nil {
		f.StopCh <- struct{}{}
	}
	return true
}

// NewTicker creates fake ticker that fires when ch has value.
func NewTicker(ch <-chan time.Time) timeutil.Ticker {
	return &FakeTicker{Ch: ch}
}

type FakeTicker struct {
	Ch      <-chan time.Time
	stopped bool
	m       sync.RWMutex
}

func (f *FakeTicker) setStopped(stopped bool) {
	f.m.Lock()
	defer f.m.Unlock()
	f.stopped = stopped
}

func (f *FakeTicker) getStopped() bool {
	f.m.RLock()
	defer f.m.RUnlock()
	return f.stopped
}

func (f *FakeTicker) C() <-chan time.Time   { return f.Ch }
func (f *FakeTicker) Stop()                 { f.setStopped(true) }
func (f *FakeTicker) Reset(d time.Duration) { f.setStopped(false) }
func (f *FakeTicker) IsStopped() bool       { return f.getStopped() }
