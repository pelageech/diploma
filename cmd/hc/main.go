package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pelageech/diploma/healthcheck"
	workerpool "github.com/pelageech/diploma/schedule/worker-pool"
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
				t := rand.N(time.Second / 2)
				time.Sleep(t)
				atomic.AddInt32(&ii[0], 1)
				return ctx.Err()
			}),
			Timeout:  time.Second,
			Interval: 1 * time.Second,
		},
		{
			Backend: healthcheck.NewBackend(func(ctx context.Context) error {
				t := rand.N(time.Second / 2)
				time.Sleep(t)
				atomic.AddInt32(&ii[0], 1)
				return ctx.Err()
			}),
			Timeout:  time.Second,
			Interval: 4 * time.Second,
		},
		{
			Backend: healthcheck.NewBackend(func(ctx context.Context) error {
				t := rand.N(time.Second / 2)
				time.Sleep(t)
				atomic.AddInt32(&ii[0], 1)
				return ctx.Err()
			}),
			Timeout:  time.Second,
			Interval: 8 * time.Second,
		},
		{
			Backend: healthcheck.NewBackend(func(ctx context.Context) error {
				t := rand.N(time.Second / 2)
				time.Sleep(t)
				atomic.AddInt32(&ii[0], 1)
				return ctx.Err()
			}),
			Timeout:  time.Second,
			Interval: 16 * time.Second,
		},
		{
			Backend: healthcheck.NewBackend(func(ctx context.Context) error {
				t := rand.N(time.Second / 2)
				time.Sleep(t)
				atomic.AddInt32(&ii[0], 1)
				return ctx.Err()
			}),
			Timeout:  time.Second,
			Interval: 32 * time.Second,
		},
	}
	ntasks := 5000
	tasks := os.Getenv("N_TASKS_REPEAT")
	_ntasks, err := strconv.Atoi(tasks)
	if err == nil {
		ntasks = _ntasks
	}
	h := healthcheck.NewHealthcheck(slices.Repeat(targets, ntasks), workerpool.NewScheduler(nil))

	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
			case <-t.C:
				fmt.Println(atomic.LoadInt32(&ii[0]))
			}
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
