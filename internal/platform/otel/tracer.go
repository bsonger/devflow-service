package otel

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Config struct {
	Endpoint           string
	Protocol           string
	ServiceName        string
	ResourceAttributes string
	SampleRatio        float64
}

func InitOtel(ctx context.Context, config *Config) (func(context.Context) error, error) {
	cfg := resolveConfig(config)
	if cfg.Endpoint == "" {
		return nil, errors.New("otel endpoint is required")
	}
	if cfg.ServiceName == "" {
		return nil, errors.New("service name is required")
	}

	exporter, err := newExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}

	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(newSampler(cfg)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	logger.Logger.Info(
		"OpenTelemetry tracing initialized",
		zap.String("service", cfg.ServiceName),
		zap.String("endpoint", cfg.Endpoint),
		zap.String("protocol", cfg.Protocol),
		zap.Float64("sample_ratio", sampleRatio(cfg.SampleRatio)),
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
		semconv.ServiceVersion(logger.ServiceVersion()),
		semconv.DeploymentEnvironmentName(logger.Environment()),
		attribute.String("k8s.cluster.name", getEnv("CLUSTER_NAME", "unknown")),
		attribute.String("k8s.namespace.name", getEnv("POD_NAMESPACE", "default")),
		attribute.String("k8s.pod.name", getEnv("POD_NAME", "unknown")),
		attribute.String("k8s.container.name", getEnv("CONTAINER_NAME", "unknown")),
		attribute.String("k8s.node.name", getEnv("NODE_NAME", "unknown")),
	}
	attrs = append(attrs, parseResourceAttributes(cfg.ResourceAttributes)...)

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
	if value <= 0 {
		return 0.1
	}
	if value > 1 {
		return 1.0
	}
	return value
}

func resolveConfig(cfg *Config) *Config {
	if cfg == nil {
		cfg = &Config{}
	}

	resolved := *cfg
	if resolved.Endpoint == "" {
		resolved.Endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	if resolved.Protocol == "" {
		resolved.Protocol = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"))
	}
	if resolved.Protocol == "" {
		resolved.Protocol = "grpc"
	}
	if resolved.ServiceName == "" {
		resolved.ServiceName = strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	}
	if resolved.ServiceName == "" {
		resolved.ServiceName = logger.ServiceName()
	}
	if resolved.ResourceAttributes == "" {
		resolved.ResourceAttributes = strings.TrimSpace(os.Getenv("OTEL_RESOURCE_ATTRIBUTES"))
	}
	if resolved.SampleRatio == 0 {
		resolved.SampleRatio = envFloat64("OTEL_TRACES_SAMPLER_ARG")
	}

	return &resolved
}

func newExporter(ctx context.Context, cfg *Config) (sdktrace.SpanExporter, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Protocol)) {
	case "", "grpc":
		endpoint, insecure := normalizeGRPCEndpoint(cfg.Endpoint)
		options := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithTimeout(5 * time.Second),
		}
		if insecure {
			options = append(options, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.New(ctx, options...)
	case "http/protobuf":
		endpoint, path, insecure := normalizeHTTPEndpoint(cfg.Endpoint)
		options := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithURLPath(path),
			otlptracehttp.WithTimeout(5 * time.Second),
		}
		if insecure {
			options = append(options, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, options...)
	default:
		return nil, errors.New("unsupported otel protocol: " + cfg.Protocol)
	}
}

func newSampler(cfg *Config) sdktrace.Sampler {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER"))) {
	case "", "parentbased_traceidratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio(cfg.SampleRatio)))
	case "always_on":
		return sdktrace.AlwaysSample()
	case "always_off":
		return sdktrace.NeverSample()
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio(cfg.SampleRatio)))
	}
}

func parseResourceAttributes(value string) []attribute.KeyValue {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	attrs := make([]attribute.KeyValue, 0, len(parts))
	for _, part := range parts {
		key, raw, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		raw = strings.TrimSpace(raw)
		if key == "" || raw == "" {
			continue
		}
		attrs = append(attrs, attribute.String(key, raw))
	}
	return attrs
}

func envFloat64(key string) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func normalizeGRPCEndpoint(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" {
		return trimmed, true
	}
	return u.Host, u.Scheme != "https"
}

func normalizeHTTPEndpoint(raw string) (endpoint string, path string, insecure bool) {
	trimmed := strings.TrimSpace(raw)
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" {
		return trimmed, "/v1/traces", true
	}

	path = strings.TrimSpace(u.Path)
	if path == "" || path == "/" {
		path = "/v1/traces"
	}
	return u.Host, path, u.Scheme != "https"
}
