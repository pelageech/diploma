package simple_time_effective

import (
	"context"
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
	mu      sync.RWMutex
	jobs    map[stand.JobID]*stand.Job
	timeMap map[time.Duration]map[stand.JobID]struct{}
}

func NewScheduler(jobs []*stand.Job) *Scheduler {
	s := &Scheduler{}
	m := make(map[stand.JobID]*stand.Job, len(jobs))
	s.timeMap = make(map[time.Duration]map[stand.JobID]struct{}, len(jobs))

	for _, j := range jobs {
		_ = s.addThreadUnsafe(j)

	}
	s.jobs = m
	return s
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

	wg := sync.WaitGroup{}
	q := queue.NewPool(5000, queue.WithWorkerCount(450))
	q.Start()

	defer q.Release()

	wg.Add(len(s.jobs))
	for dur, jobs := range s.timeMap {
		wg.Add(1)
		go func(dur time.Duration, jobs map[stand.JobID]struct{}) {
			defer wg.Done()
			ticker := time.NewTicker(dur)
			defer ticker.Stop()

			for range ticker.C {
				select {
				case <-ctx.Done():
					return
				default:
				}
				s.mu.RLock()
				for jobID := range jobs {
					job := s.jobs[jobID]
					tout := job.Timeout()
					tasksWaiting.Add(ctx, 1)
					err := q.QueueTask(func(ctx context.Context) error {
						t := time.Now()
						defer taskFullPath.Record(ctx, int64(time.Since(t).Milliseconds()))
						tasksWaiting.Add(ctx, -1)
						return job.Run(ctx)
					}, job2.AllowOption{Timeout: &tout})
					if err != nil {
						log.Println(job.ID(), err)
					}
				}
				s.mu.RUnlock()
			}
		}(dur, jobs)

	}

	wg.Wait()
	return nil
}

func (s *Scheduler) Add(job *stand.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addThreadUnsafe(job)
}

func (s *Scheduler) addThreadUnsafe(job *stand.Job) error {
	m, ok := s.timeMap[job.Interval()]
	if !ok {
		m = make(map[stand.JobID]struct{})
		s.timeMap[job.Interval()] = m
	}
	if _, ok := m[job.ID()]; ok {
		return stand.ErrJobExists
	}
	m[job.ID()] = struct{}{}
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
