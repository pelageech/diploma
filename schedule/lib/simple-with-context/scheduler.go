package simple_with_context

import (
	"context"
	"iter"
	"log"
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

func (s *Scheduler) Schedule(ctx context.Context) {
	wg := sync.WaitGroup{}
	wg.Add(len(s.jobs))
	for _, job := range s.jobs {
		go func() {
			defer wg.Done()

			ticker := time.NewTicker(job.Interval())
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					break
				default:
				}

				select {
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(ctx, job.Timeout())
					if err := job.Run(ctx); err != nil {
						log.Println(err)
					}
					cancel()
				}
			}
		}()
	}

	wg.Wait()
}

func (s *Scheduler) Add(job *stand.Job) {
	s.jobs[job.ID()] = job
}

func (s *Scheduler) Remove(id stand.JobID) {
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
