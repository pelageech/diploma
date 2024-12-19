package sched

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pelageech/diploma/schedule/timeutil"
)

func TestClockConcurrentAdd(t *testing.T) {
	c := Clock{
		OnTickDrop: func(p time.Duration) {
			t.Fatalf("unexpected tick drop for %s", p)
		},
	}
	defer c.Stop()
	var (
		span = time.Millisecond
		noop = TaskFunc(func(now time.Time) {})
	)
	for i := 0; i < 8; i++ {
		go func() {
			for i := 0; i < 100; i++ {
				if _, err := c.Add(span, noop); err != nil {
					return
				}
				runtime.Gosched()
			}
		}()
	}
	time.Sleep(span * 5)
}

type StubTimer struct {
	t          testing.TB
	shiftTime  func(time.Duration)
	cleanup    func()
	createOnce sync.Once

	expect []chan struct{}

	timeCh  chan time.Time
	resetCh chan time.Duration
	stopCh  chan struct{}

	active int32
}

func NewStubTimer(t testing.TB) *StubTimer {
	shiftTime, cleanup := timeutil.StubTestHookTimeNow(time.Unix(0, 0))
	return &StubTimer{
		t: t,

		shiftTime: shiftTime,
		cleanup:   cleanup,

		timeCh:  make(chan time.Time),
		resetCh: make(chan time.Duration),
		stopCh:  make(chan struct{}),
		active:  1,
	}
}

func (s *StubTimer) Reset(p time.Duration) bool {
	s.resetCh <- p
	return atomic.SwapInt32(&s.active, 1) == 1
}

func (s *StubTimer) Stop() bool {
	s.stopCh <- struct{}{}
	return atomic.SwapInt32(&s.active, 0) == 1
}

func (s *StubTimer) C() <-chan time.Time {
	return s.timeCh
}

func (s *StubTimer) MustReset(exp time.Duration) {
	act := <-s.resetCh
	if act != exp {
		s.t.Fatalf("unexpected reset duration: %v; want %v", act, exp)
	}
	s.shiftTime(act)
}

func (s *StubTimer) MustStop() {
	<-s.stopCh
}

func (s *StubTimer) ExpectStop(n int) {
	exp := make(chan struct{})
	s.expect = append(s.expect, exp)
	go func() {
		defer close(exp)
		for i := 0; i < n; i++ {
			<-s.stopCh
		}
	}()
}

func (s *StubTimer) Fire() {
	atomic.StoreInt32(&s.active, 0)
	s.timeCh <- timeutil.Now()
}

func (s *StubTimer) Close() {
	for _, exp := range s.expect {
		<-exp
	}
	s.cleanup()
}

func (s *StubTimer) TimerFunc() func(time.Duration) timeutil.Timer {
	return func(time.Duration) timeutil.Timer {
		var ok bool
		s.createOnce.Do(func() {
			ok = true
		})
		if !ok {
			s.t.Fatalf("getting timer more than once")
		}
		return s
	}
}

func TestClockAdd(t *testing.T) {
	type event struct {
		handle int
		moment time.Time
	}
	for _, test := range []struct {
		name    string
		ticks   int
		handles int
	}{
		{
			name:    "one",
			handles: 1,
			ticks:   100,
		},
		{
			name:    "two",
			handles: 2,
			ticks:   100,
		},
		{
			name:    "eight",
			handles: 8,
			ticks:   100,
		},
		{
			name:    "many",
			handles: 1000,
			ticks:   100,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			timer := NewStubTimer(t)
			defer timer.Close()

			// Expect first Stop() during initialization of timer.
			timer.ExpectStop(1)

			c := &Clock{
				NewTimerFunc: timer.TimerFunc(),
				OnTickDrop: func(p time.Duration) {
					t.Errorf("unexpected tick drop for %q", p)
				},
			}
			var (
				timeline = make(chan event, test.handles*test.ticks)
				handles  = make([]TaskHandle, test.handles)
			)
			for i := range handles {
				var err error
				i := i
				handles[i], err = c.Add(1, TaskFunc(func(now time.Time) {
					timeline <- event{i, now}
				}))
				if err != nil {
					t.Fatal(err)
				}
				if i == 0 {
					timer.MustReset(1)
				}
			}
			for i := 0; i < test.ticks; i++ {
				timer.Fire()
				timer.MustReset(1)

				for j := 0; j < test.handles; j++ {
					exp := event{
						handle: j,
						moment: time.Unix(0, int64(i+1)),
					}
					act := <-timeline
					if act != exp {
						t.Errorf("unexpected #%d event: %v; want %v", i, act, exp)
					}
				}
			}
			for _, h := range handles {
				c.Del(h)
			}
		})
	}
}

func TestClockDelWithinTask(t *testing.T) {
	timer := NewStubTimer(t)
	defer timer.Close()
	timer.ExpectStop(2)

	c := &Clock{
		NewTimerFunc: timer.TimerFunc(),
		OnTickDrop: func(time.Duration) {
			t.Fatalf("unexpected tick drop")
		},
	}
	defer c.Stop()

	deleted := make(chan struct{})
	var handle TaskHandle
	handle, _ = c.Add(1, TaskFunc(func(time.Time) {
		defer close(deleted)
		c.Del(handle)
	}))
	timer.MustReset(1)
	timer.Fire()
	timer.MustReset(1)

	select {
	case <-deleted:
		t.Fatalf("unexpected unlock")
	case <-time.After(time.Second):
	}
}

func TestClockDel(t *testing.T) {
	for _, test := range []struct {
		name   string
		single bool
	}{
		//{single: false},
		{single: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			const (
				timeout = time.Millisecond * 100
			)
			var (
				entered = make(chan struct{})
				release = make(chan struct{})
				deleted = make(chan struct{})
			)

			timer := NewStubTimer(t)
			defer timer.Close()
			timer.ExpectStop(2)

			c := &Clock{
				NewTimerFunc: timer.TimerFunc(),
				OnTickDrop: func(time.Duration) {
					t.Fatalf("unexpected tick drop")
				},
			}
			defer c.Stop()

			handle, _ := c.Add(1, TaskFunc(func(time.Time) {
				entered <- struct{}{}
				<-release
			}))
			timer.MustReset(1)

			if !test.single {
				c.Add(1, TaskFunc(func(time.Time) {
					// noop
				}))
			}

			// Trigger task execution. Block the execution.
			timer.Fire()
			timer.MustReset(1)
			<-entered

			go func() {
				defer close(deleted)
				c.Del(handle)
			}()

			select {
			case <-deleted:
				t.Fatalf("unexpected Del() return")
			case <-time.After(timeout):
			}

			// Release task execution. After this Del() should return.
			close(release)
			select {
			case <-deleted:
			case <-time.After(timeout):
				t.Fatalf("no Del() return")
			}
		})
	}
}

func TestClockDropTick(t *testing.T) {
	timer := NewStubTimer(t)
	defer timer.Close()
	timer.ExpectStop(1)

	var (
		handled = make(chan struct{}, 3)
		drop    = make(chan struct{})
		sleep   = make(chan struct{})
	)
	c := &Clock{
		NewTimerFunc: timer.TimerFunc(),
		OnTickDrop: func(time.Duration) { // Will panic when called twice.
			close(sleep)
			close(drop)
		},
	}
	c.Add(1, TaskFunc(func(t time.Time) {
		// Will block until next tick drop.
		handled <- struct{}{}
		<-sleep
	}))
	timer.MustReset(1)

	const timeout = 50 * time.Millisecond

	timer.Fire() // Block the tick handler.
	timer.MustReset(1)

	select {
	case <-drop:
		t.Fatalf("unexpected tick drop")
	case <-handled:
		// OK.
	}

	timer.Fire() // Fill the tick buffer.
	timer.MustReset(1)

	select {
	case <-drop:
		t.Fatalf("unexpected tick drop")
	case <-time.After(timeout):
		// OK.
	}

	timer.Fire() // This tick must be dropped.
	timer.MustReset(1)

	select {
	case <-drop:
		// OK.
	case <-time.After(timeout):
		t.Fatalf("no dropped ticks after %s", timeout)
	}
}

func TestClockStop(t *testing.T) {
	timer := NewStubTimer(t)
	defer timer.Close()
	timer.ExpectStop(1)

	var (
		ticked  = make(chan struct{})
		stopped = make(chan struct{})
	)
	c := &Clock{
		NewTimerFunc: timer.TimerFunc(),
	}
	c.Add(time.Hour, TaskFunc(func(time.Time) {
		close(ticked) // Will panic on second call.
	}))
	timer.MustReset(time.Hour)
	timer.Fire()
	timer.MustReset(time.Hour)
	<-ticked // Wait for tick to be received.
	go func() {
		defer close(stopped)
		c.Stop()
	}()
	timer.MustStop()
	<-stopped
}
