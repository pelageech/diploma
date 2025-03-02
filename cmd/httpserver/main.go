package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func Hello(w http.ResponseWriter, req *http.Request) {
	d, err := time.ParseDuration(req.URL.Query().Get("dur"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	time.Sleep(d)
	io.WriteString(w, "Hello World")
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	s := http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(Hello),
	}
	go func() {
		log.Printf("listening on %s", s.Addr)
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	s.Shutdown(context.Background())
}
