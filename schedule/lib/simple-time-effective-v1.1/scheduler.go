package simple_time_effective

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"sync"
	"time"

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
	meter := otel.GetMeterProvider().Meter("scheduler")

	tasksWaiting, _ := meter.Int64UpDownCounter("tasks_waiting")
	//tasksTimeout, _ := meter.Int64Counter("tasks_timeout")
	taskFullPath, _ := meter.Int64Histogram("task_full_path")
	jobsCount, _ := meter.Int64UpDownCounter("jobs_count")

	wg := sync.WaitGroup{}
	jobCh := make(chan *stand.Job, 50000)
	closed := false
	mu := &sync.RWMutex{}
	go func() {
		select {
		case <-ctx.Done():
		}
		mu.Lock()
		defer mu.Unlock()

		closed = true
		close(jobCh)
	}()
	wg.Add(len(s.jobs))
	m := map[stand.JobID]chan struct{}{}
	for _, job := range s.jobs {

		//done := make(chan struct{})
		m[job.ID()] = make(chan struct{}, 1)
		//jobCh := make(chan *stand.Job)
		go func(job *stand.Job) {
			defer jobsCount.Add(context.Background(), -1)
			jobsCount.Add(context.Background(), 1)
			defer wg.Done()
			//defer func() {
			//	close(jobCh)
			//}()
			ticker := time.NewTicker(job.Interval())
			defer ticker.Stop()

			<-ticker.C
			timer := time.NewTimer(job.Timeout())
			defer timer.Stop()

			mu.RLock()
			if closed {
				mu.RUnlock()
				return
			}
			jobCh <- job
			mu.RUnlock()
			tasksWaiting.Add(ctx, 1)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				<-m[job.ID()]
				<-ticker.C
				mu.RLock()
				if closed {
					mu.RUnlock()
					return
				}
				jobCh <- job
				mu.RUnlock()
				tasksWaiting.Add(ctx, 1)
			}
		}(job)
	}
	for range 460 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				select {
				case <-ctx.Done():
					for job := range jobCh {
						close(m[job.ID()])
					}
					return
				default:
				}
				t := time.Now()
				tasksWaiting.Add(ctx, -1)
				err := job.Run(ctx)
				if err != nil {
					slog.Error("job err", "id", job.ID(), "err", err)
				}
				m[job.ID()] <- struct{}{}
				taskFullPath.Record(ctx, int64(time.Since(t).Milliseconds()))
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
