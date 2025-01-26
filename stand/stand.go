package stand

import (
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/otel"
	"iter"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

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
	timeout       time.Duration
	interval      time.Duration
	id            JobID
	task          Runnable
	hist          metric.Int64Histogram
	tasksComplete metric.Int64Counter
	tasksWorking  metric.Int64UpDownCounter

	// +checkatomic
	runCount int64
}

func (j *Job) ID() JobID {
	return j.id
}

var ErrJobTimeout = errors.New("job timeout")

type runConfig struct {
	cancellable bool
	immediately bool
}

type JobRunOpt func(*runConfig)

// WithCancellable creates a new context with timeout on the Run.
// If deadline is exceeded, the context is cancelled.
//
// If the runner provider returns an error of the context, it SHOULD
// return it with context.Cause in other to catch ErrJobTimeout which
// is provided when the context is cancelled.
func WithCancellable() JobRunOpt {
	return func(j *runConfig) {
		j.cancellable = true
	}
}

// WithReturnImmediately sets the behaviour of the Run func to
// return immediately when context is cancelled.
//
// Warning: Run will create an extra goroutine to provide such a behaviour.
// If your function is able to return immediately by catching context, use
// WithCancellable instead.
func WithReturnImmediately() JobRunOpt {
	return func(j *runConfig) {
		j.immediately = true
	}
}

// WithJobRunCount is under construction.
func WithJobRunCount(count int64, block bool) JobRunOpt {
	return func(j *runConfig) {}
}

var (
	_attributeTimeout = metric.WithAttributes(attribute.String("status", "TIMEOUT"))
	_attributeErr     = metric.WithAttributes(attribute.String("status", "ERR"))
	_attributeOK      = metric.WithAttributes(attribute.String("status", "OK"))
)

func (j *Job) Run(ctx context.Context, opts ...JobRunOpt) error {
	c := &runConfig{}
	for _, opt := range opts {
		opt(c)
	}

	t := time.Now()
	j.tasksWorking.Add(ctx, 1)

	var err error
	if c.cancellable {
		var cancel func()
		ctx, cancel = context.WithTimeoutCause(ctx, j.timeout, ErrJobTimeout)
		defer cancel()
	}
	if c.immediately {
		done := make(chan struct{})
		go func() {
			defer close(done)
			err = j.run(ctx)
		}()
		select {
		case <-done:
		case <-ctx.Done():
		}
	} else {
		err = j.run(ctx)
	}

	j.tasksWorking.Add(ctx, -1)
	f := time.Since(t).Milliseconds()

	if err != nil {
		if errors.Is(err, ErrJobTimeout) {
			j.hist.Record(ctx, f, _attributeTimeout)
			return err
		}

		j.hist.Record(ctx, f, _attributeErr)
		return err
	}
	j.tasksComplete.Add(ctx, 1)
	j.hist.Record(ctx, f, _attributeOK)

	return nil
}

func (j *Job) run(ctx context.Context) error {
	atomic.AddInt64(&j.runCount, 1)
	err := j.task.Run(ctx)
	atomic.AddInt64(&j.runCount, -1)
	return err
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

func (j *Job) Task() Runnable {
	return j.task
}

type JobOpt func(*Job)

func WithOtelTimingHist(hist metric.Int64Histogram) JobOpt {
	return func(j *Job) {
		j.hist = hist
	}
}

func NewJob(task Runnable, timeout, interval time.Duration, opts ...JobOpt) *Job {
	tasksComplete, err := otel.Meter("scheduler").Int64Counter("tasks_complete")
	if err != nil {
		panic(err)
	}

	hist, err := otel.Meter("scheduler").Int64Histogram("job_timings")
	if err != nil {
		panic(err)
	}

	tasksWorking, err := otel.Meter("scheduler").Int64UpDownCounter("tasks_working")
	if err != nil {
		panic(err)
	}

	j := &Job{
		timeout:       timeout,
		interval:      interval,
		id:            generateID(),
		task:          task,
		hist:          hist,
		tasksComplete: tasksComplete,
		tasksWorking:  tasksWorking,
	}

	for _, opt := range opts {
		opt(j)
	}

	return j
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
