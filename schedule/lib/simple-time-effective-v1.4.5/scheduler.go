package simple_time_effective

import (
	"context"
	"fmt"
	job2 "github.com/golang-queue/queue/job"
	"iter"
	"log"
	"sync"
	"time"

	"github.com/golang-queue/queue"
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
	q := queue.NewPool(5000, queue.WithWorkerCount(458))
	q.Start()

	defer q.Release()

	wg.Add(len(s.jobs))
	for _, job := range s.jobs {
		j := true
		//done := make(chan struct{})
		//jobCh := make(chan *stand.Job)
		go func(job *stand.Job) {
			tout := job.Timeout()
			defer jobsCount.Add(context.Background(), -1)
			jobsCount.Add(context.Background(), 1)
			defer wg.Done()
			ticker := time.NewTicker(job.Interval())
			defer ticker.Stop()

			for {
				<-ticker.C
				select {
				case <-ctx.Done():
					return
				default:
				}
				tasksWaiting.Add(ctx, 1)
				err := q.QueueTask(func(ctx context.Context) error {
					t := time.Now()
					defer taskFullPath.Record(ctx, int64(time.Since(t).Milliseconds()))
					tasksWaiting.Add(ctx, -1)
					return job.Run(ctx)
				}, job2.AllowOption{Timeout: &tout, Jitter: &j})
				if err != nil {
					log.Println(job.ID(), err)
				}
			}
		}(job)
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
