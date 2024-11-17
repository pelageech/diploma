package main

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_, span := otel.GetTracerProvider().Tracer("").Start(ctx, "aboba")
			tm := 10*time.Millisecond + rand.N(100*time.Millisecond)
			time.Sleep(tm)
			span.SetAttributes(attribute.String("time_dur", tm.String()))
			span.End()
		}
	}()
	<-ctx.Done()
}
