package stand

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/google/uuid"
)

type JobID fmt.Stringer

func GenerateID() JobID {
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

func NewJob(task Runnable, timeout, interval time.Duration) *Job {
	return &Job{
		timeout:  timeout,
		interval: interval,
		id:       GenerateID(),
		task:     task,
	}
}

type Scheduler interface {
	Jobs() iter.Seq2[JobID, *Job]
	Schedule(context.Context)
	Add(*Job)
	Remove(JobID)
}

type BatchScheduler interface {
	Scheduler
	BatchAdd(iter.Seq[*Job])
	BatchRemove(iter.Seq[*Job])
}
