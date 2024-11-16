package main

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx := context.Background()
	l := slog.Default()
	stop, err := prepare(ctx)
	if err != nil {
		slog.Default().Error("prepare", "err", err)
		return
	}

	defer func() {
		l.Info("closing...")
		if err != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			stopErr := stop(ctx)
			if stopErr != nil {
				l.Error("stop after err failed", "err", stopErr)
			}
		}
	}()

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	<-ctx.Done()
}
