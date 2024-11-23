package main

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"log/slog"
	"math/rand/v2"
	"os/signal"
	"slices"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pelageech/diploma/healthcheck"
	ste "github.com/pelageech/diploma/schedule/lib/simple-time-effective"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	shutdown, err := prepare(ctx)
	if err != nil {
		slog.Error("error preparing shutdown:", "err", err)
		return
	}
	defer func() {
		err := shutdown(context.Background())
		if err != nil {
			slog.Error("error shutting down:", "err", err)
		}
	}()

	var ii [3]int32
	targets := []*healthcheck.Target{
		{
			Backend: healthcheck.NewBackend(func(ctx context.Context) error {
				ctx, span := otel.GetTracerProvider().Tracer("main-2").Start(ctx, "healthcheck")
				defer span.End()

				t := rand.N(time.Second / 2)
				time.Sleep(t)
				span.SetAttributes(attribute.Int64("time", int64(t)))
				ctx, span = otel.GetTracerProvider().Tracer("main-2").Start(ctx, "healthcheck.atomic")
				defer span.End()
				atomic.AddInt32(&ii[0], 1)
				return ctx.Err()
			}),
			Timeout:  time.Second,
			Interval: 1 * time.Second,
		},
		{
			Backend: healthcheck.NewBackend(func(ctx context.Context) error {
				ctx, span := otel.GetTracerProvider().Tracer("main-2").Start(ctx, "healthcheck")
				defer span.End()

				t := rand.N(time.Second / 2)
				time.Sleep(t)
				span.SetAttributes(attribute.Int64("time", int64(t)))
				ctx, span = otel.GetTracerProvider().Tracer("main-2").Start(ctx, "healthcheck.atomic")
				defer span.End()
				atomic.AddInt32(&ii[1], 1)
				return ctx.Err()
			}),
			Timeout:  time.Second,
			Interval: 4 * time.Second,
		},
		{
			Backend: healthcheck.NewBackend(func(ctx context.Context) error {
				ctx, span := otel.GetTracerProvider().Tracer("main-2").Start(ctx, "healthcheck")
				defer span.End()

				t := rand.N(time.Second / 2)
				time.Sleep(t)
				span.SetAttributes(attribute.Int64("time", int64(t)))
				ctx, span = otel.GetTracerProvider().Tracer("main-2").Start(ctx, "healthcheck.atomic")
				defer span.End()
				atomic.AddInt32(&ii[2], 1)
				return ctx.Err()
			}),
			Timeout:  time.Second,
			Interval: 8 * time.Second,
		},
	}
	h := healthcheck.NewHealthcheck(slices.Repeat(targets, 10000), ste.NewScheduler(nil))

	go func() {
		for {
			select {
			case <-ctx.Done():
			default:
			}
			<-time.After(1 * time.Second)
			fmt.Println(atomic.LoadInt32(&ii[0]))
		}
	}()

	done := make(chan struct{}, 1)
	go func() {
		h.Start(ctx)
		done <- struct{}{}
		slog.Info("scheduler is shut down")
	}()

	<-done
}
