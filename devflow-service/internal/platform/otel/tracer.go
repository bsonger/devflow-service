package otelx

import (
	"context"
	"errors"
	"os"
	"time"

	loggingx "github.com/bsonger/devflow-service/internal/platform/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Config struct {
	Endpoint    string
	ServiceName string
	SampleRatio float64
}

func InitOtel(ctx context.Context, config *Config) (func(context.Context) error, error) {
	if config.Endpoint == "" {
		return nil, errors.New("otel endpoint is required")
	}
	if config.ServiceName == "" {
		return nil, errors.New("service name is required")
	}

	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(config.Endpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, err
	}

	res, err := buildResource(ctx, config)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(
			sdktrace.ParentBased(
				sdktrace.TraceIDRatioBased(sampleRatio(config.SampleRatio)),
			),
		),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	loggingx.Logger.Info(
		"OpenTelemetry tracing initialized",
		zap.String("service", config.ServiceName),
		zap.String("endpoint", config.Endpoint),
		zap.Float64("sample_ratio", sampleRatio(config.SampleRatio)),
	)

	shutdown := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(ctx)
	}

	return shutdown, nil
}

func Start(ctx context.Context, tracerName, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, spanName, opts...)
}

func buildResource(ctx context.Context, cfg *Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceNamespace(getEnv("POD_NAMESPACE", "default")),
		semconv.ServiceVersion(getEnv("SERVICE_VERSION", "unknown")),
		semconv.DeploymentEnvironmentName(getEnv("ENV", "unknown")),
		attribute.String("k8s.cluster.name", getEnv("CLUSTER_NAME", "unknown")),
		attribute.String("k8s.namespace.name", getEnv("POD_NAMESPACE", "default")),
		attribute.String("k8s.pod.name", getEnv("POD_NAME", "unknown")),
		attribute.String("k8s.container.name", getEnv("CONTAINER_NAME", "unknown")),
		attribute.String("k8s.node.name", getEnv("NODE_NAME", "unknown")),
	}

	return resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(attrs...),
	)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func sampleRatio(value float64) float64 {
	if value <= 0 || value > 1 {
		return 1.0
	}
	return value
}
