package healthcheck

import (
	"context"
	"github.com/panjf2000/ants/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
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

var pool, _ = ants.NewPool(ants.DefaultAntsPoolSize)

type check struct {
	f               func(context.Context) error
	stillProcessing metric.Int64UpDownCounter
	tasksTimeout    metric.Int64Counter
}

func (c check) Run(ctx context.Context) error {
	return c.f(ctx)
}

func (c check) Exec(_ time.Time) {
	//err := c(context.Background())
	//if err != nil {
	//	panic(err)
	//}
	go func() {
		ctx, cancel := context.WithTimeoutCause(context.Background(), time.Second, stand.ErrJobTimeout)
		defer cancel()

		c.stillProcessing.Add(ctx, 1)
		err := c.f(ctx)
		c.stillProcessing.Add(ctx, -1)
		if err != nil {
			c.tasksTimeout.Add(ctx, 1)
		}
	}()
}

func NewHealthcheck(targets []*Target, scheduler stand.BatchScheduler) *Healthcheck {
	h := &Healthcheck{
		targets:   targets,
		scheduler: scheduler,
	}
	hist, _ := otel.Meter("main").Int64Histogram("job_timings")
	stillProcessing, _ := otel.Meter("scheduler").Int64UpDownCounter("still_processing")
	tasksTimeout, _ := otel.Meter("scheduler").Int64Counter("tasks_timeout")
	scheduler.BatchAdd(func(yield func(*stand.Job) bool) {
		for _, target := range targets {
			job := stand.NewJob(
				check{
					f:               target.Backend.check,
					stillProcessing: stillProcessing,
					tasksTimeout:    tasksTimeout,
				},
				target.Timeout,
				target.Interval,
				stand.WithOtelTimingHist(hist),
			)
			if !yield(job) {
				return
			}
		}
	})

	return h
}
