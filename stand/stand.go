package stand

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"time"

	"github.com/google/uuid"
)

type JobID fmt.Stringer

func generateID() JobID {
	return uuid.New()
}

type Runnable interface {
	Run(ctx context.Context) error
}

type Job struct {
	timeout  time.Duration
	interval time.Duration
	id       JobID
	task     Runnable
}

func (j *Job) ID() JobID {
	return j.id
}

func (j *Job) Run(ctx context.Context) error {
	return j.task.Run(ctx)
}

func (j *Job) Timeout() time.Duration {
	return j.timeout
}

func (j *Job) Interval() time.Duration {
	return j.interval
}

func (j *Job) Clone() *Job {
	return &Job{
		timeout:  j.timeout,
		interval: j.interval,
		id:       generateID(),
		task:     j.task,
	}
}

func NewJob(task Runnable, timeout, interval time.Duration) *Job {
	return &Job{
		timeout:  timeout,
		interval: interval,
		id:       generateID(),
		task:     task,
	}
}

var (
	ErrJobExists   = errors.New("job already exists")
	ErrJobNotFound = errors.New("job not found")
)

// Scheduler is the interface for schedule periodically executing jobs. It contains the necessary minimum
// providing the Schedule function and an adaptivity.
type Scheduler interface {
	// Jobs returns an iterator to all the jobs that the scheduler processes currentry.
	Jobs() iter.Seq2[JobID, *Job]

	// Schedule starts scheduling tasks containing in the pool. It should be run synchronously.
	//
	// The function should return an error if the scheduling can't be completed anymore.
	// Context cancellation should provide graceful shutdown for all the running jobs and then
	// return an error from the context using [context.Cause].
	//
	// Task errors don't influent Scheduler error.
	Schedule(context.Context) error

	// Add adds a new job into the pool to schedule. If job's ID collides with
	// another job contaning in the pool, the function should return ErrJobExists.
	Add(*Job) error

	// Remove finds the job with a given ID and pulls it from the schedulong pool.
	// If there's no the job with such ID, ErrJobNotFound is returned.
	Remove(JobID) error
}

type BatchScheduler interface {
	Scheduler
	BatchAdd(iter.Seq[*Job])
	BatchRemove(iter.Seq[*Job])
}

type PrepareScheduler interface {
	Scheduler
	Prepare(context.Context) error
}
