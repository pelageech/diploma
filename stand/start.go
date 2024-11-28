package stand

import (
	"context"
	"slices"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type Stand struct {
	scheduler Scheduler
	tracer    trace.Tracer
	meter     metric.Meter
}

type Starter func(context.Context) error

func New(scheduler Scheduler) *Stand {
	return &Stand{
		scheduler: scheduler,
		tracer:    otel.GetTracerProvider().Tracer("default_tracer"),
		meter:     otel.GetMeterProvider().Meter("default_meter"),
	}
}

func (s *Stand) InitJobN(j *Job, n int) Starter {
	jobs := make([]*Job, n)
	for i := range n {
		jobs[i] = j.Clone()
	}
	return func(ctx context.Context) error {
		err := s.generalInit(ctx, jobs)
		if err != nil {
			return err
		}

		return s.scheduler.Schedule(ctx)
	}
}

func (s *Stand) generalInit(ctx context.Context, jobsIter []*Job) error {
	ctx, span := s.tracer.Start(ctx, "starter.scheduler.init")
	defer span.End()

	span.SetAttributes(attribute.Int("job_len", len(jobsIter)))

	sched := s.scheduler
	if psched, ok := sched.(PrepareScheduler); ok {
		span.SetAttributes(attribute.Bool("is_prepare", true))
		ctx, span := s.tracer.Start(ctx, "starter.scheduler.prepare")
		err := psched.Prepare(ctx)
		span.RecordError(err)
		span.End()

		if err != nil {
			return err
		}
	} else {
		span.SetAttributes(attribute.Bool("is_prepare", false))
	}

	if bsched, ok := sched.(BatchScheduler); ok {
		bsched.BatchAdd(slices.Values(jobsIter))
	} else {
		for _, job := range jobsIter {
			err := sched.Add(job)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
