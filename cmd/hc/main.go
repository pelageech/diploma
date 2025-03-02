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
	"net/url"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/outcaste-io/ristretto/z"
	"github.com/pelageech/diploma/healthcheck"
)

func alloc[T any](z *z.Allocator) *T {
	var t T
	return (*T)(unsafe.Pointer(&z.Allocate(int(unsafe.Sizeof(t)))[0]))
}

type task struct {
	t   *time.Ticker
	tt  *time.Timer
	ii  [1]*int32
	cfg time.Duration
}

func (t *task) Run(ctx context.Context) error {
	if t.t == nil {
		t.t = time.NewTicker(time.Millisecond / 8)
	} else {
		t.t.Reset(time.Millisecond / 8)
	}
	if t.tt == nil {
		t.tt = time.NewTimer(rand.N(t.cfg))
	} else {
		t.tt.Reset(rand.N(t.cfg))
	}
	i := 0
loop:
	for range t.t.C {
		select {
		case <-t.tt.C:
			break loop
		default:
		}
		i++
	}

	//if t.tt == nil {
	//	t.tt = time.NewTimer(rand.N(t.cfg))
	//} else {
	//	t.tt.Reset(rand.N(t.cfg))
	//}
	//<-t.tt.C

	atomic.AddInt32(t.ii[0], 1)
	return nil
}

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

	var a int32 = 0
	var ii = [1]*int32{&a}

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
			for range targetCfg.Count {
				u, err := url.Parse("127.0.0.1:8080/abcd?dur=" + (20*time.Millisecond + rand.N(targetCfg.Sleep)).String())
				if err != nil {
					panic(err)
				}
				//t := healthcheck.NewTCPCheck("127.0.0.1:8080")
				t := &healthcheck.HTTPCheck{
					Url:    u,
					Client: http.DefaultClient,
				}
				t.OnDone = func() {
					atomic.AddInt32(ii[0], 1)
				}
				targets = append(targets, &healthcheck.Target{
					Backend:  healthcheck.NewBackend(t),
					Timeout:  targetCfg.Timeout,
					Interval: targetCfg.Interval,
				})
			}
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
				fmt.Println(atomic.LoadInt32(ii[0]))
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
