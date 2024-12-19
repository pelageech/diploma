package ingoroutine

import (
	"context"
	"errors"
	"fmt"
	"github.com/pelageech/diploma/stand"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"iter"
	"sync"
	"time"
)

type Scheduler struct {
	jobs map[stand.JobID]*stand.Job
}

func NewScheduler(jobs []*stand.Job) *Scheduler {
	m := make(map[stand.JobID]*stand.Job, len(jobs))
	for _, j := range jobs {
		m[j.ID()] = j
	}
	return &Scheduler{jobs: m}
}

func (s *Scheduler) Jobs() iter.Seq2[stand.JobID, *stand.Job] {
	return func(yield func(stand.JobID, *stand.Job) bool) {
		for k, v := range s.jobs {
			if !yield(k, v) {
				return
			}
		}
	}
}

func (s *Scheduler) Add(job *stand.Job) error {
	if _, ok := s.jobs[job.ID()]; ok {
		return fmt.Errorf("%s: %w", job.ID(), stand.ErrJobExists)
	}
	s.jobs[job.ID()] = job
	return nil
}

func (s *Scheduler) Remove(id stand.JobID) error {
	//TODO implement me
	panic("implement me")
}

func (s *Scheduler) BatchAdd(jobs iter.Seq[*stand.Job]) {
	for job := range jobs {
		s.Add(job)
	}
}

func (s *Scheduler) BatchRemove(i iter.Seq[*stand.Job]) {
	//TODO implement me
	panic("implement me")
}

func (s *Scheduler) Schedule(ctx context.Context) error {
	meter := otel.Meter("scheduler")

	jobsCount, _ := meter.Int64UpDownCounter("jobs_count")
	tasksTimeout, _ := meter.Int64Counter("tasks_timeout")
	taskFullPath, _ := meter.Int64Histogram("task_full_path")
	stillProcessing, _ := meter.Int64UpDownCounter("still_processing")

	wg := sync.WaitGroup{}
	wg.Add(len(s.jobs))
	for _, job := range s.jobs {
		go func(ctx context.Context, job *stand.Job) {
			generalAttrs := metric.WithAttributes(
				attribute.Int64("interval", job.Interval().Milliseconds()))

			defer jobsCount.Add(ctx, -1)
			jobsCount.Add(ctx, 1)
			defer wg.Done()
			ticker := time.NewTicker(job.Interval())
			defer ticker.Stop()

			for {
				t := time.Now()

				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					stillProcessing.Add(ctx, 1, generalAttrs)
					err := job.Run(ctx, stand.WithCancellable())
					stillProcessing.Add(ctx, -1, generalAttrs)

					if errors.Is(err, stand.ErrJobTimeout) {
						tasksTimeout.Add(ctx, 1, generalAttrs)
					}
				}

				taskFullPath.Record(ctx, time.Since(t).Milliseconds(), generalAttrs)
			}
		}(ctx, job)
	}

	wg.Wait()
	return nil
}
