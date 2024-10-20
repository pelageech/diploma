package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"
	"sync/atomic"
	"time"

	"github.com/pelageech/diploma/healthcheck"
	ste "github.com/pelageech/diploma/schedule/lib/simple-time-effective"
)

func Handler(ii *int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(ii, 1)
	}
}

func main() {
	var ii int32
	done := make(chan struct{}, 1)
	go func() {
		l, err := net.Listen("tcp", ":8080")
		if err != nil {
			panic(err)
		}
		defer l.Close()
		done <- struct{}{}
		mux := http.NewServeMux()
		mux.HandleFunc("/", Handler(&ii))
		s := http.Server{
			Handler: mux,
		}
		http.HandleFunc("GET /", Handler(&ii))

		if err := s.Serve(l); err != nil {
			panic(err)
		}
	}()

	<-done
	targets := []*healthcheck.Target{
		{
			Backend: healthcheck.NewBackend(func(ctx context.Context) error {
				t := http.DefaultTransport.(*http.Transport)
				t.MaxIdleConns = 1000
				t.MaxConnsPerHost = 1000
				t.MaxIdleConnsPerHost = 1000
				c := &http.Client{
					Transport: t,
					Timeout:   time.Second,
				}
				resp, err := c.Get("http://127.0.0.1:8080/")
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("bad status: %s", resp.Status)
				}
				return ctx.Err()
			}),
			Timeout:  time.Second / 2,
			Interval: 1 * time.Second,
		},
	}
	h := healthcheck.NewHealthcheck(slices.Repeat(targets, 9000), ste.NewScheduler(nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			<-time.After(2 * time.Second)
			fmt.Println(atomic.LoadInt32(&ii))
		}
	}()
	go h.Start(ctx)
	select {}
}
