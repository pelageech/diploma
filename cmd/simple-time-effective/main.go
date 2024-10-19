package main

import (
	"context"
	"log"
	"sync"
	"time"
)

type Runner interface {
	Timeout() time.Duration
	Interval() time.Duration
	Run(ctx context.Context) error
}

type Job struct {
	Tout time.Duration
	Ival time.Duration
}

func (j Job) Timeout() time.Duration {
	return j.Tout
}

func (j Job) Interval() time.Duration {
	return j.Ival
}

func (j Job) Run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-time.After(j.Timeout() - time.Second):
		return nil
	}
}

type Scheduler[R Runner] struct {
	Jobs []R
}

func (s Scheduler[_]) Start(ctx context.Context) error {
	wg := sync.WaitGroup{}
	wg.Add(len(s.Jobs))
	ch := make(chan Runner, 1000000)
	go func() {
		for {
			<-time.After(time.Second)
			log.Println(len(ch))
		}
	}()
	for _, job := range s.Jobs {
		go func() {
			defer wg.Done()

			timer := time.NewTimer(job.Timeout())
			defer timer.Stop()

			ticker := time.NewTicker(job.Interval())
			defer ticker.Stop()

			for {
				timer.Reset(job.Timeout())
				select {
				case ch <- job:
				case <-timer.C:
					panic(":(")
				}
				if !timer.Stop() {
					<-timer.C
				}
				<-ticker.C
			}
		}()
	}

	wg.Wait()
	return nil
}

func Repeat[T any, S ~[]T](s S, n int) S {
	if n <= 0 {
		panic("n must be > 0")
	}
	res := make(S, n*len(s))
	for i := range n {
		copy(res[i*len(s):(i+1)*len(s)], s)
	}
	return res
}

func main() {
	tasks := Repeat([]Job{{
		Tout: 3 * time.Second,
		Ival: 5 * time.Second,
	}}, 200000)

	sch := Scheduler[Job]{Jobs: tasks}
	sch.Start(context.Background())
}
