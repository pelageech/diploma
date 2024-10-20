package healthcheck

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/pelageech/diploma/stand"
)

type Backend struct {
	alive atomic.Bool
	check func(ctx context.Context) error
	inRow int32
}

func NewBackend(check func(ctx context.Context) error) Backend {
	return Backend{
		check: check,
	}
}

func (b *Backend) InRow() int32 {
	return atomic.LoadInt32(&b.inRow)
}

type Target struct {
	Backend  Backend
	Timeout  time.Duration
	Interval time.Duration
}

type Healthcheck struct {
	targets   []*Target
	scheduler stand.Scheduler
}

func (h *Healthcheck) Start(ctx context.Context) {
	h.scheduler.Schedule(ctx)
}

type check func(context.Context) error

func (c check) Run(ctx context.Context) error {
	return c(ctx)
}

func NewHealthcheck(targets []*Target, scheduler stand.BatchScheduler) *Healthcheck {
	h := &Healthcheck{
		targets:   targets,
		scheduler: scheduler,
	}
	scheduler.BatchAdd(func(yield func(*stand.Job) bool) {
		for _, target := range targets {
			job := stand.NewJob(
				check(target.Backend.check),
				target.Timeout,
				target.Interval,
			)
			if !yield(job) {
				return
			}
		}
	})

	return h
}
