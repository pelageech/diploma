package cmd

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func closer(fns ...func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) (err error) {
		for _, fn := range fns {
			err = errors.Join(err, fn(ctx))
		}
		return err
	}
}

func Prepare(ctx context.Context) (stop func(context.Context) error, err error) {
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("runtime-instrumentation-example"),
	))
	if err != nil {
		return nil, err
	}

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
	//
	//stopTrace, err := grpcTracePrep(ctx, res)
	//if err != nil {
	//	return closer(stop, stopTrace), err
	//}
	//stop = closer(stop, stopTrace)
	//
	//err = otelRuntimeMeter()
	//if err != nil {
	//	return stop, err
	//}

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
	}), otlpmetricgrpc.WithEndpoint("127.0.0.1:4317"))
	if err != nil {
		return nil, err
	}
	stop := exp.Shutdown

	read := metric.NewPeriodicReader(exp, metric.WithInterval(1*time.Second))
	stop = closer(read.Shutdown)

	provider := metric.NewMeterProvider(metric.WithResource(res), metric.WithReader(read))
	otel.SetMeterProvider(provider)
	stop = closer(stop, provider.Shutdown)

	return stop, nil
}

func grpcTracePrep(ctx context.Context, res *resource.Resource) (func(context.Context) error, error) {
	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithTimeout(10*time.Second), otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig{
		Enabled:         true,
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		MaxElapsedTime:  10 * time.Second,
	}), otlptracegrpc.WithEndpoint("192.168.0.3:4317"))
	if err != nil {
		return nil, err
	}
	stop := exp.Shutdown

	otel.SetTextMapPropagator(newPropagator())
	provider := trace.NewTracerProvider(trace.WithResource(res), trace.WithBatcher(exp))
	stop = closer(stop, provider.Shutdown)
	otel.SetTracerProvider(provider)

	return stop, err
}

func otelRuntimeMeter() error {
	return runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second))
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}
