package healthcheck

import (
	"context"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/pelageech/diploma/stand"
)

type Backend struct {
	alive atomic.Bool
	inRow int32
}

func (b *Backend) Check(ctx context.Context) error {
	time.Sleep(50*time.Millisecond + rand.N(450*time.Millisecond))
	return nil
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
				check(target.Backend.Check),
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
