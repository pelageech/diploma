package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/pelageech/diploma/healthcheck"
	ste "github.com/pelageech/diploma/schedule/lib/simple-time-effective"
)

func main() {
	targets := []*healthcheck.Target{
		{
			Backend:  healthcheck.Backend{},
			Timeout:  time.Second,
			Interval: 1 * time.Second,
		},
		{
			Backend:  healthcheck.Backend{},
			Timeout:  time.Second,
			Interval: 5 * time.Second,
		},
		{
			Backend:  healthcheck.Backend{},
			Timeout:  time.Second,
			Interval: 10 * time.Second,
		},
	}
	h := healthcheck.NewHealthcheck(slices.Repeat(targets, 50000), ste.NewScheduler(nil))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		for {
			<-time.After(1 * time.Second)
			fmt.Println(targets[0].Backend.InRow())
		}
	}()
	go h.Start(ctx)
	select {}
}
