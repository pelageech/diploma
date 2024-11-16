package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

func closer(fns ...func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) (err error) {
		for _, fn := range fns {
			err = errors.Join(err, fn(ctx))
		}
		return err
	}
}

func prepare(ctx context.Context) (stop func(context.Context) error, err error) {
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("runtime-instrumentation-example"),
	)

	stop, err = grpcPrep(ctx, res)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			stopErr := stop(context.Background())
			if stopErr != nil {
				slog.Default().Error("stop after err failed", "err", stopErr)
			}
		}
	}()

	err = otelRuntimeMeter()
	if err != nil {
		return stop, err
	}

	return stop, nil
}

func stdout(res *resource.Resource) (func(context.Context) error, error) {
	exp, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
	if err != nil {
		return nil, err
	}
	stop := closer(exp.Shutdown)

	read := metric.NewPeriodicReader(exp, metric.WithInterval(1*time.Second))
	stop = closer(stop, read.Shutdown)

	provider := metric.NewMeterProvider(metric.WithResource(res), metric.WithReader(read))
	otel.SetMeterProvider(provider)
	stop = closer(stop, provider.Shutdown)

	return stop, nil
}

func grpcPrep(ctx context.Context, res *resource.Resource) (func(context.Context) error, error) {
	exp, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithInsecure(), otlpmetricgrpc.WithTimeout(10*time.Second), otlpmetricgrpc.WithRetry(otlpmetricgrpc.RetryConfig{
		Enabled:         true,
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		MaxElapsedTime:  10 * time.Second,
	}), otlpmetricgrpc.WithEndpoint("192.168.0.3:4317"))
	if err != nil {
		return nil, err
	}
	slog.Default().Info("connected")
	stop := exp.Shutdown

	read := metric.NewPeriodicReader(exp, metric.WithInterval(1*time.Second))
	stop = closer(read.Shutdown)

	provider := metric.NewMeterProvider(metric.WithResource(res), metric.WithReader(read))
	otel.SetMeterProvider(provider)
	stop = closer(stop, provider.Shutdown)

	return stop, nil
}

func otelRuntimeMeter() error {
	return runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second))
}
