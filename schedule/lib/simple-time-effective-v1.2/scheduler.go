package simple_time_effective

import (
	"container/heap"
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
	jobs map[stand.JobID]*Node
	jh   heap.Interface
	mu   sync.RWMutex
}

func NewScheduler(jobs []*stand.Job) *Scheduler {
	m := make(map[stand.JobID]*Node, len(jobs))
	var h heap.Interface = &JobsHeap{}
	for _, j := range jobs {
		n := &Node{
			key: 0,
			val: j,
		}
		m[j.ID()] = n
		heap.Push(h, n)
	}
	heap.Init(h)
	return &Scheduler{jobs: m, jh: h}
}

func (s *Scheduler) Jobs() iter.Seq2[stand.JobID, *stand.Job] {
	return func(yield func(stand.JobID, *stand.Job) bool) {
		for k, v := range s.jobs {
			if !yield(k, v.val) {
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
	closed := false
	mu := &sync.RWMutex{}
	go func() {
		select {
		case <-ctx.Done():
		}
		mu.Lock()
		defer mu.Unlock()

		closed = true
	}()
	wg.Add(len(s.jobs))
	m := map[stand.JobID]chan struct{}{}
	for _, job := range s.jobs {

		//done := make(chan struct{})
		m[job.val.ID()] = make(chan struct{}, 1)
		//jobCh := make(chan *stand.Job)
		go func(job *Node) {
			defer jobsCount.Add(context.Background(), -1)
			jobsCount.Add(context.Background(), 1)
			defer wg.Done()
			//defer func() {
			//	close(jobCh)
			//}()
			ticker := time.NewTicker(job.val.Interval())
			defer ticker.Stop()

			<-ticker.C
			timer := time.NewTimer(job.val.Timeout())
			defer timer.Stop()

			mu.RLock()
			if closed {
				mu.RUnlock()
				return
			}
			s.mu.Lock()
			job.key++
			heap.Fix(s.jh, job.pos)
			s.mu.Unlock()
			mu.RUnlock()
			tasksWaiting.Add(ctx, 1)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				<-m[job.val.ID()]
				<-ticker.C
				mu.RLock()
				if closed {
					mu.RUnlock()
					return
				}
				s.mu.Lock()
				job.key++
				heap.Fix(s.jh, job.pos)
				s.mu.Unlock()
				mu.RUnlock()
				tasksWaiting.Add(ctx, 1)
			}
		}(job)
	}
	for range 460 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				s.mu.Lock()
				node := heap.Pop(s.jh).(*Node)
				heap.Push(s.jh, &Node{
					key: node.key - 1,
					val: node.val,
					pos: node.pos,
				})
				s.mu.Unlock()
				job := node.val
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
	n := &Node{
		key: 0,
		val: job,
		pos: 0,
	}
	s.jobs[job.ID()] = n
	heap.Push(s.jh, n)
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
