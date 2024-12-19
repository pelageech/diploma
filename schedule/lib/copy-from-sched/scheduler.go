package sched

import (
	"context"
	"fmt"
	"github.com/pelageech/diploma/schedule/timeutil"
	"github.com/pelageech/diploma/stand"
	"go.opentelemetry.io/otel"
	"iter"
	"time"
)

type Scheduler struct {
	clock   *Clock
	jobs    map[stand.JobID]*stand.Job
	handles map[stand.JobID]TaskHandle
}

func (s *Scheduler) Jobs() iter.Seq2[stand.JobID, *stand.Job] {
	return func(yield func(stand.JobID, *stand.Job) bool) {
		for jobID, job := range s.jobs {
			if !yield(jobID, job) {
				return
			}
		}
	}
}

func (s *Scheduler) Schedule(ctx context.Context) error {
	<-ctx.Done()
	s.clock.Stop()
	return nil
}

func (s *Scheduler) Add(job *stand.Job) error {
	t, ok := job.Task().(Task)
	if !ok {
		return fmt.Errorf("cannot add task to scheduler, job: %v", job)
	}
	handle, err := s.clock.Add(job.Interval(), t)
	if err != nil {
		return err
	}
	s.handles[job.ID()] = handle
	return nil
}

func (s *Scheduler) Remove(id stand.JobID) error {
	handle, ok := s.handles[id]
	if !ok {
		return stand.ErrJobNotFound
	}

	return s.clock.Del(handle)
}

func NewScheduler(jobs []*stand.Job) (*Scheduler, error) {
	tasksTimeout, _ := otel.Meter("scheduler").Int64Counter("tasks_timeout")
	clock := &Clock{
		OnTickDrop: func(duration time.Duration) {
			tasksTimeout.Add(context.Background(), 1)
		},
		NewTimerFunc: func(duration time.Duration) timeutil.Timer {
			return timeutil.NewTimer(duration)
		},
	}
	s := &Scheduler{
		clock:   clock,
		jobs:    make(map[stand.JobID]*stand.Job, len(jobs)),
		handles: make(map[stand.JobID]TaskHandle, len(jobs)),
	}
	for _, job := range jobs {
		if err := s.Add(job); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Scheduler) BatchAdd(jobs iter.Seq[*stand.Job]) {
	for job := range jobs {
		err := s.Add(job)
		if err != nil {
			panic(err)
		}
	}
}

func (s *Scheduler) BatchRemove(i iter.Seq[*stand.Job]) {
	//TODO implement me
	panic("implement me")
}
