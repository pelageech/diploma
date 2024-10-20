package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"sync/atomic"
	"time"

	"github.com/pelageech/diploma/healthcheck"
	ste "github.com/pelageech/diploma/schedule/lib/simple-time-effective"
)

func main() {
	var ii [3]int32
	targets := []*healthcheck.Target{
		{
			Backend: healthcheck.NewBackend(func(ctx context.Context) error {
				time.Sleep(rand.N(time.Second / 2))
				atomic.AddInt32(&ii[0], 1)
				return ctx.Err()
			}),
			Timeout:  time.Second,
			Interval: 1 * time.Second,
		},
		{
			Backend: healthcheck.NewBackend(func(ctx context.Context) error {
				atomic.AddInt32(&ii[1], 1)
				return ctx.Err()
			}),
			Timeout:  time.Second,
			Interval: 4 * time.Second,
		},
		{
			Backend: healthcheck.NewBackend(func(ctx context.Context) error {
				atomic.AddInt32(&ii[2], 1)
				return ctx.Err()
			}),
			Timeout:  time.Second,
			Interval: 8 * time.Second,
		},
	}
	h := healthcheck.NewHealthcheck(slices.Repeat(targets, 150000), ste.NewScheduler(nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			<-time.After(1 * time.Second)
			fmt.Println(atomic.LoadInt32(&ii[0]))
		}
	}()
	go h.Start(ctx)
	select {}
}
