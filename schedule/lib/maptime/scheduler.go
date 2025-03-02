package maptime

import (
	"context"
	"github.com/pelageech/diploma/schedule/lib/base"
	"github.com/pelageech/diploma/stand"
	"log/slog"
	"sync"
	"time"
)

type Scheduler struct {
	*base.Scheduler

	mu sync.RWMutex
	//+checklocks:mu
	timeMap map[time.Duration]map[stand.JobID]struct{}

	cond *sync.Cond
	//+checklocks:mu
	running map[time.Duration]bool

	chJobs chan *stand.Job
}

func NewScheduler(jobs []*stand.Job) *Scheduler {
	s := &Scheduler{
		Scheduler: base.NewScheduler(jobs),
		timeMap:   make(map[time.Duration]map[stand.JobID]struct{}),
		running:   make(map[time.Duration]bool),
		chJobs:    make(chan *stand.Job, len(jobs)),
	}

	s.cond = sync.NewCond(&s.mu)

	for _, job := range jobs {
		// single thread, run unsafe func to speed up the constructor
		_ = s.addThreadUnsafe(job)
	}
	return s
}

func (s *Scheduler) Schedule(ctx context.Context) error {
	go func() {
		s.cond.L.Lock()
		defer s.cond.L.Unlock()

		for {
			if ctx.Err() != nil {
				return
			}
			for dur, ok := range s.running {
				if ok {
					continue
				}
				go s.runMain(ctx, dur)
			}
			s.cond.Wait()
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
		}

		s.mu.Lock()
		defer s.mu.Unlock()
		close(s.chJobs)
	}()

	const N = 450

	for range N {
		go func() {
			for job := range s.chJobs { // do not select, close chan on exit
				if ctx.Err() != nil {
					return
				}
				err := job.Run(ctx)
				if err != nil {
					slog.Error("job err", "id", job.ID(), "err", err)
				}
			}
		}()
	}

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
		s.running[job.Interval()] = false
		s.cond.Signal()
	}
	if _, ok := m[job.ID()]; ok {
		return stand.ErrJobExists
	}
	m[job.ID()] = struct{}{}
	s.Scheduler.JobsMap()[job.ID()] = job
	return nil
}

func (s *Scheduler) runMain(ctx context.Context, dur time.Duration) {
	ticker := time.NewTicker(dur)
	defer ticker.Stop()

	for {
		<-ticker.C
		if ctx.Err() != nil {
			return
		}
		s.mu.RLock()
		for id := range s.timeMap[dur] {
			s.chJobs <- s.Scheduler.JobsMap()[id]
		}
		s.mu.RUnlock()
	}
}
