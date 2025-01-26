package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"github.com/pelageech/diploma/schedule/config"
	"github.com/pelageech/diploma/stand"
	"log"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"slices"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pelageech/diploma/healthcheck"
)

var configPath = flag.String("config", "./cmd/hc/config.yaml", "path to config file")

func main() {

	flag.Parse()

	configFile, err := os.Open(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	defer configFile.Close()

	cfg, err := config.Export(configFile)
	if err != nil {
		slog.Error("failed to export config", "err", err)
		os.Exit(1)
	}
	slog.Info("cfg", "cfg", cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	shutdown, err := prepare(ctx)
	if err != nil {
		slog.Error("error preparing shutdown:", "err", err)
		os.Exit(1)
	}
	defer func() {
		err := shutdown(context.Background())
		if err != nil {
			slog.Error("error shutting down:", "err", err)
		}
	}()

	var ii [3]int32

	var (
		targets   []*healthcheck.Target
		scheduler stand.Scheduler
	)
	if cfg.Version == "v1" {
		scheduler, err = config.ToScheduler(cfg.Scheduler.Type, []*stand.Job(nil))
		if err != nil {
			slog.Error("failed to convert scheduler config", "err", err)
			os.Exit(1)
		}
		for _, targetCfg := range cfg.Scheduler.Targets {
			targets = append(targets, slices.Repeat([]*healthcheck.Target{{
				Backend: healthcheck.NewBackend(func(ctx context.Context) error {

					//ctxTime, cancel := context.WithTimeout(ctx, rand.N(targetCfg.Sleep))
					//defer cancel()
					tm := time.NewTimer(rand.N(targetCfg.Sleep))
					defer tm.Stop()
					t := time.NewTicker(time.Millisecond / 8)
					defer t.Stop()
					i := 0
				loop:
					for {
						select {
						case <-tm.C:
							break loop
						case <-t.C:
						}
						i++
					}

					atomic.AddInt32(&ii[0], 1)
					return context.Cause(ctx)
				}),
				Timeout:  targetCfg.Timeout,
				Interval: targetCfg.Interval,
			}}, targetCfg.Count)...)
		}
	} else {
		slog.Info("scheduler version unsupported", "version", cfg.Version)
		os.Exit(1)
	}

	h := healthcheck.NewHealthcheck(targets, scheduler.(stand.BatchScheduler))

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

	done := make(chan struct{}, 2)
	go func() {
		h.Start(ctx)
		done <- struct{}{}
		slog.Info("scheduler is shut down")
	}()

	s := http.Server{
		Addr: "localhost:8080",
	}
	go func() {
		l, _ := net.Listen("tcp", ":8080")
		if err := s.Serve(l); err != http.ErrServerClosed {
			return
		}
	}()

	<-done
	<-ctx.Done()
	_ = s.Shutdown(ctx)
}
