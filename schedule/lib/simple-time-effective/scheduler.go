package simple_time_effective

import (
	"context"
	"fmt"
	"iter"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/pelageech/diploma/stand"
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
	wg := sync.WaitGroup{}
	wg.Add(len(s.jobs))
	//ch := make(chan *stand.Job, 1000000)
	for _, job := range s.jobs {
		done := make(chan struct{})
		jobCh := make(chan *stand.Job)
		go func() {
			defer wg.Done()
			defer func() {
				close(jobCh)
			}()
			timer := time.NewTimer(job.Timeout())
			defer timer.Stop()

			ticker := time.NewTicker(job.Interval())
			defer ticker.Stop()

			jobCh <- job
			for {
				timer.Reset(job.Timeout())
				select {
				case <-ctx.Done():
					return
				case <-done:
				case <-timer.C:
					log.Println("job timeout:", job.ID())
				}
				if !timer.Stop() {
					<-timer.C
				}

				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					jobCh <- job
				}
			}
		}()
		go func() {
			for {
				select {
				case <-ctx.Done():
					close(done)
					return
				case job := <-jobCh:
					err := job.Run(ctx)
					if err != nil {
						slog.Error("job err", "id", job.ID(), "err", err)
					}
					done <- struct{}{}
				}
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

func (s *Scheduler) BatchRemove(i iter.Seq[*stand.Job]) {
	//TODO implement me
	panic("implement me")
}
