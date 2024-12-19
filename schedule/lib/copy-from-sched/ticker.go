package sched

import (
	"container/heap"
	"strconv"
	"sync"
	"time"

	"github.com/pelageech/diploma/schedule/timeutil"
)

var zeroTime time.Time

type ticker struct {
	NewTimerFunc func(time.Duration) timeutil.Timer

	spans *spanTimers
	timer timeutil.Timer
	next  time.Time

	once    sync.Once
	done    chan struct{}
	control chan spanControl
}

func (t *ticker) init() {
	t.once.Do(func() {
		t.done = make(chan struct{})
		t.control = make(chan spanControl)
		t.spans = newSpanTimers()
		t.timer = t.newTimer()
		go t.run()
	})
}

func (t *ticker) Tick(p time.Duration, f func(now time.Time)) (stop func()) {
	t.init()
	t.control <- spanControl{
		f: f,
		p: p,
		t: spanAdd,
	}
	return func() {
		t.control <- spanControl{
			p: p,
			t: spanDel,
		}
	}
}

func (t *ticker) Stop() {
	t.init()
	close(t.control)
	<-t.done
}

func (t *ticker) active() bool {
	return !t.next.IsZero()
}

func (t *ticker) stopTimer() {
	if t.active() && !t.timer.Stop() {
		<-t.timer.C()
	}
	t.next = zeroTime
}

func (t *ticker) nextTick() time.Time {
	if t.spans.Len() == 0 {
		return zeroTime
	}
	_, when, _ := t.spans.Head()
	return when
}

func (t *ticker) resetTimer(next time.Time) {
	if t.next.Equal(next) {
		return
	}
	t.next = next
	t.timer.Reset(timeutil.Until(next))
}

func (t *ticker) tick(now time.Time) {
	t.next = zeroTime
	for t.spans.Len() > 0 {
		p, when, f := t.spans.Head()
		if when.After(now) {
			break
		}
		f(now)
		t.spans.ResetHead(now.Add(p))
	}
}

func (t *ticker) run() {
	defer func() {
		t.stopTimer()
		close(t.done)
	}()
	for {
		select {
		case c, ok := <-t.control:
			if !ok {
				return
			}
			switch c.t {
			case spanAdd:
				t.spans.Insert(c.p, c.f)
			case spanDel:
				t.spans.Delete(c.p)
			default:
				panic("unknown span control type: " + strconv.Itoa(int(c.t)))
			}
			t.stopTimer()

		case now := <-t.timer.C():
			t.tick(now)
		}
		if next := t.nextTick(); !next.IsZero() {
			t.resetTimer(next)
		}
	}
}

func (t *ticker) newTimer() (timer timeutil.Timer) {
	const aLongTime = time.Duration(1<<63 - 1)
	if f := t.NewTimerFunc; f != nil {
		timer = f(aLongTime)
	} else {
		timer = timeutil.NewTimer(aLongTime)
	}
	if !timer.Stop() {
		panic("sched: unexpected timer firing")
	}
	return
}

type spanControlType uint

const (
	spanAdd spanControlType = iota
	spanDel
)

type spanControl struct {
	f func(time.Time)
	p time.Duration
	t spanControlType
}

type spanTimer struct {
	f      func(time.Time)
	period time.Duration
	when   time.Time
}

type spanTimers struct {
	index map[time.Duration]int
	spans []*spanTimer
}

func newSpanTimers() *spanTimers {
	return &spanTimers{
		index: make(map[time.Duration]int),
	}
}

func (s *spanTimers) Len() int {
	return len(s.spans)
}

func (s *spanTimers) Less(i, j int) bool {
	a := s.spans[i]
	b := s.spans[j]
	return a.when.Before(b.when)
}

func (s *spanTimers) Swap(i, j int) {
	a := s.spans[i]
	b := s.spans[j]
	s.index[a.period] = j
	s.index[b.period] = i
	s.spans[i] = b
	s.spans[j] = a
}

func (s *spanTimers) Push(x interface{}) {
	t := x.(*spanTimer)
	s.index[t.period] = len(s.spans)
	s.spans = append(s.spans, t)
}

func (s *spanTimers) Pop() interface{} {
	n := len(s.spans)
	t := s.spans[n-1]
	s.spans = s.spans[:n-1]
	delete(s.index, t.period)
	return t
}

func (s *spanTimers) Insert(p time.Duration, f func(time.Time)) {
	_, has := s.index[p]
	if has {
		panic("trying to add previously added span")
	}
	s.index[p] = -1
	heap.Push(s, &spanTimer{
		f:      f,
		period: p,
		when:   timeutil.Now().Add(p),
	})
}

func (s *spanTimers) Delete(p time.Duration) {
	i, has := s.index[p]
	if !has {
		panic("trying to del not added span")
	}
	delete(s.index, p)
	heap.Remove(s, i)
}

func (s *spanTimers) ResetHead(when time.Time) {
	s.spans[0].when = when
	heap.Fix(s, 0)
}

func (s *spanTimers) Head() (p time.Duration, when time.Time, f func(time.Time)) {
	x := s.spans[0]
	return x.period, x.when, x.f
}
