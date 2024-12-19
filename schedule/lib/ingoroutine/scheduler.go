package ingoroutine

import (
	"context"
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
			defer jobsCount.Add(ctx, -1)
			jobsCount.Add(ctx, 1)
			defer wg.Done()
			ticker := time.NewTicker(job.Interval())
			defer ticker.Stop()

			timer := time.NewTimer(job.Timeout())
			defer timer.Stop()

			for {
				t := time.Now()

				if !timer.Stop() {
					<-timer.C
				}
				done := make(chan struct{}, 1)

				select {
				case <-ctx.Done():
					return
				case <-ticker.C:

					timer.Reset(job.Timeout())
					go func() {
						stillProcessing.Add(ctx, 1, metric.WithAttributes(
							attribute.Int64("interval", job.Interval().Milliseconds())))
						_ = job.Run(ctx)
						stillProcessing.Add(ctx, -1, metric.WithAttributes(
							attribute.Int64("interval", job.Interval().Milliseconds())))

						select {
						case done <- struct{}{}:
						default:
						}
					}()

					select {
					case <-ctx.Done():
						//p.Store(&onDoneStub)
						return
					case <-timer.C:
						//p.Store(&onDoneStub)
						tasksTimeout.Add(ctx, 1)
					case <-done:
					}
				}
				done = nil
				//select {
				//case <-done:
				//default:
				//}
				taskFullPath.Record(ctx, time.Since(t).Milliseconds(), metric.WithAttributes(
					attribute.Int64("interval", job.Interval().Milliseconds())))
			}
		}(ctx, job)
	}

	wg.Wait()
	return nil
}
