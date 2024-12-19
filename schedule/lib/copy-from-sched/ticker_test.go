package sched

import (
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/pelageech/diploma/schedule/timeutil"
)

func TestSpanTimers(t *testing.T) {
	for _, test := range []struct {
		name string
		in   []int
		del  []int
		exp  []int
	}{
		{
			in:  []int{3, 1, 0, 2},
			exp: []int{0, 1, 2, 3},
		},
		{
			in:  []int{3, 1, 0, 2},
			del: []int{3, 0},
			exp: []int{1, 2},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Need to "stop the time", because timers heap compares the time
			// to fire, not the timer periods.
			_, cleanup := timeutil.StubTestHookTimeNow(time.Unix(0, 0))
			defer cleanup()

			s := newSpanTimers()
			for _, p := range test.in {
				s.Insert(time.Duration(p), nil)
			}
			for _, p := range test.del {
				s.Delete(time.Duration(p))
			}

			var act []int
			for s.Len() > 0 {
				p, _, _ := s.Head()
				s.Delete(p)
				act = append(act, int(p))
			}
			if exp := test.exp; !cmp.Equal(act, exp) {
				t.Fatalf(
					"unexpected timers order: %v; want %v",
					act, exp,
				)
			}
		})
	}
}

func TestSpanTimersResetHead(t *testing.T) {
	// Need to "stop the time", because timers heap compares the time
	// to fire, not the timer periods.
	_, cleanup := timeutil.StubTestHookTimeNow(time.Unix(0, 0))
	defer cleanup()

	s := newSpanTimers()
	s.Insert(time.Second, nil)
	s.Insert(time.Minute, nil)

	assertHead := func(expP time.Duration, expWhen time.Time) {
		actP, actWhen, _ := s.Head()
		if actP != expP {
			t.Fatalf("unexpected head's period: %s; want %s", actP, expP)
		}
		if !actWhen.Equal(expWhen) {
			t.Fatalf("unexpected head's when: %s; want %s", actWhen, expWhen)
		}
	}

	now := timeutil.Now()
	assertHead(time.Second, now.Add(time.Second))

	s.ResetHead(now.Add(time.Hour))
	assertHead(time.Minute, now.Add(time.Minute))

	s.ResetHead(now.Add(time.Hour * 2))
	assertHead(time.Second, now.Add(time.Hour))
}

func f(numRoutines int) {
	a := int64(0)
	var wg sync.WaitGroup
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func() {
			for range 300 {
				atomic.AddInt64(&a, 1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func gplus(numRoutines int) {
	var wg sync.WaitGroup
	wg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func() {
			time.Sleep(10 * time.Second)
			wg.Done()
		}()
	}
	wg.Wait()
}

func g(numRoutines int) {
	a := int64(0)
	var wg sync.WaitGroup
	wg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func() {
			defer wg.Done()
			for range 300 {
				atomic.AddInt64(&a, 1)
			}
		}()
	}
	wg.Wait()
}

func BenchmarkWG(b *testing.B) {
	debug.SetMemoryLimit(1 << 30)
	b.ResetTimer()
	for _, n := range []int{1, 1000, 10000, 100000, 1000000} {
		b.Run("g_="+strconv.Itoa(n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				g(n)
			}
		})
		b.Run("f_="+strconv.Itoa(n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				f(n)
			}
		})
	}
}
