package sched

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/pelageech/diploma/schedule/timeutil"
)

type TaskRun struct {
	exec time.Time
	task TaskHandle
}

func (tr *TaskRun) Run(_ context.Context) error {
	tr.task.x.Exec(tr.exec)
	return nil
}

// Task is the interface that wraps scheduled task.
type Task interface {
	Exec(time.Time)
}

// TaskFunc is an adapter to allow use of ordinary function as Task.
type TaskFunc func(time.Time)

// Exec implements Task interface.
func (f TaskFunc) Exec(t time.Time) {
	f(t)
}

// TaskHandle represents registered task in concrete clock instance.
type TaskHandle struct {
	s *span
	x *task
}

// Clock contains task scheduling logic.
type Clock struct {
	// OnTickDrop is an optional callback that will be called when some "tick"
	// of clock can not execute appropriate tasks due to high load of clock.
	OnTickDrop func(time.Duration)

	// NewTimerFunc is an optional function that will be called to construct
	// timer object. The main intention of this field is to provide way to
	// easy and non-flaky tests.
	NewTimerFunc func(time.Duration) timeutil.Timer

	mu      sync.Mutex
	once    sync.Once
	stopped bool
	index   map[time.Duration]*span
	ticker  ticker
}

// init initializes clock structures and starts necessary goroutines.
//
// It starts two goroutines: first one is to collect fired timer events and
// pass it to a second goroutine. The second goroutine is to run scheduled
// tasks for each fired timer. This is necessary to start second goroutine
// because time.Timer could drop events if its channel is filled. In that case
// we will silently loose events without any knowledge. With two goroutines
// design first one will try to receive fired timers and immediately pass them
// to the second. If that passing is blocked, it drops the timers but calls
// OnTickDrop() callback to notify the user.
func (c *Clock) init() {
	c.once.Do(func() {
		c.index = make(map[time.Duration]*span)
		c.ticker = ticker{
			NewTimerFunc: c.NewTimerFunc,
		}
	})
}

// ErrStopped is returned by Clock.Add calls when the clock is stopped by
// Clock.Stop() call.
var ErrStopped = errors.New("clock is stopped")

// Add schedules task t to be repeated with interval p.
//
// At the first time task may be executed at any moment after *now* but not
// later than *now* + p, where *now* is the moment of Add() call.
//
// Note that calling clock methods within task execution may fall in deadlock.
//
// It returns handle that can be used to remove the task from the clock.
// Returned error is non-nil if and only if the clock is stopped.
func (c *Clock) Add(p time.Duration, t Task) (TaskHandle, error) {
	c.init()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return TaskHandle{}, ErrStopped
	}
	s, has := c.index[p]
	if !has {
		s = &span{
			period: p,
			tasks:  newTaskSlice(),
		}
		s.stopTicker = c.ticker.Tick(p, func(now time.Time) {
			if !s.doer.Exec(now, s.taskList()) {
				if c.OnTickDrop != nil {
					c.OnTickDrop(p)
				}
			}
		})
		c.index[p] = s
	}
	x := newTask(t)
	s.add(x)

	return TaskHandle{s, x}, nil
}

// Del removes task from clock by given handle t.
// Given handle t must not be empty.
// It returns only when t's task done and not able to be executed anymore.
// Returned error is non-nil if and only if the clock is stopped.
func (c *Clock) Del(t TaskHandle) error {
	c.init()

	t.s.del(t.x)
	t.x.disable()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return ErrStopped
	}
	if !t.s.empty() {
		return nil
	}
	if _, has := c.index[t.s.period]; has {
		delete(c.index, t.s.period)
		t.s.stop()
	}

	return nil
}

// Stop stops all underlying timers and wait for all currently running tasks
// be completed.
// It returns only when all tasks are done.
func (c *Clock) Stop() {
	c.init()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}
	c.stopped = true
	c.index = nil

	c.ticker.Stop()

	for _, s := range c.index {
		s.stop()
	}
}

type tick struct {
	now   time.Time
	tasks *taskSlice
}

type span struct {
	period time.Duration
	tasks  *taskSlice
	doer   doer

	stopTicker func()
}

func (s *span) stop() {
	s.stopTicker()
	s.doer.Stop()
}

func (s *span) add(x *task) {
	s.tasks.PushBack(x)
}

func (s *span) del(x *task) {
	s.tasks.Remove(x)
}

func (s *span) empty() bool {
	return s.tasks.Len() == 0
}

func (s *span) taskList() *taskSlice {
	ts := s.tasks
	return ts
}
