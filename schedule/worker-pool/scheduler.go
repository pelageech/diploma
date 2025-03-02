package workerpool

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/pelageech/diploma/stand"
	"go.opentelemetry.io/otel"
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

func (s *Scheduler) Schedule(ctx context.Context) error {
	// tracer := otel.GetTracerProvider().Tracer("ste-scheduler")
	meter := otel.GetMeterProvider().Meter("ste-scheduler")

	tasksWaiting, _ := meter.Int64UpDownCounter("tasks_waiting")
	tasksTimeout, _ := meter.Int64Counter("tasks_timeout")
	tasksComplete, _ := meter.Int64Counter("tasks_complete")
	taskCompletion, _ := meter.Int64Histogram("task_full_path")

	pool, _ := ants.NewPool(1500, ants.WithPreAlloc(true), ants.WithMaxBlockingTasks(0))
	defer pool.Release()

	wg := sync.WaitGroup{}
	wg.Add(len(s.jobs))
	jobCh := make(chan *stand.Job, 500000)
	m := map[stand.JobID]chan struct{}{}
	for _, job := range s.jobs {

		//done := make(chan struct{})
		m[job.ID()] = make(chan struct{}, 1)
		//jobCh := make(chan *stand.Job)
		go func(job *stand.Job) {
			defer wg.Done()
			//defer func() {
			//	close(jobCh)
			//}()
			ticker := time.NewTicker(job.Interval())
			defer ticker.Stop()

			<-ticker.C
			timer := time.NewTimer(job.Timeout())
			defer timer.Stop()

			jobCh <- job
			tasksWaiting.Add(ctx, 1)
			for {
				select {
				case <-ctx.Done():
					return
				case <-m[job.ID()]:
				case <-timer.C:
					tasksTimeout.Add(ctx, 1)
				}
				if !timer.Stop() {
					<-timer.C
				}

				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					timer.Reset(job.Timeout())
					jobCh <- job
					tasksWaiting.Add(ctx, 1)
				}
			}
		}(job)
	}

	for range 1 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				job := job
				t := time.Now()
				_ = pool.Submit(func() {
					select {
					case <-ctx.Done():
						return
					default:
					}
					tasksWaiting.Add(ctx, -1)
					err := job.Run(ctx)
					if err != nil {
						slog.Error("job err", "id", job.ID(), "err", err)
					}
					m[job.ID()] <- struct{}{}
					tasksComplete.Add(ctx, 1)
					taskCompletion.Record(ctx, int64(time.Since(t).Milliseconds()))
				})
			}
		}()
	}

	wg.Wait()
	return nil
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

func (s *Scheduler) BatchRemove(i iter.Seq[stand.JobID]) {
	//TODO implement me
	panic("implement me")
}
