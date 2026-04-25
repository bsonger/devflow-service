package observability

import (
	"context"
	"os"
	"strings"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/internal/platform/otel"
	"github.com/bsonger/devflow-service/internal/platform/runtime/pyroscopex"
	gootel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type RuntimeOptions struct {
	LogLevel               string
	LogFormat              string
	OtelEndpoint           string
	OtelProtocol           string
	OtelService            string
	OtelResourceAttributes string
	OtelSampleRatio        float64
	PyroscopeAddr          string
	ServiceOverride        string
}

func Init(ctx context.Context, opts RuntimeOptions) (func(context.Context) error, error) {
	serviceName := ResolveServiceName(opts.ServiceOverride, opts.OtelService)
	if serviceName != "" {
		_ = os.Setenv("SERVICE_NAME", serviceName)
		if strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")) == "" {
			_ = os.Setenv("OTEL_SERVICE_NAME", serviceName)
		}
	}

	logger.InitZapLogger(&logger.Config{
		Level:  opts.LogLevel,
		Format: opts.LogFormat,
	})

	shutdown := func(context.Context) error { return nil }
	if opts.OtelEndpoint != "" {
		tpShutdown, err := otel.InitOtel(ctx, &otel.Config{
			Endpoint:           opts.OtelEndpoint,
			Protocol:           opts.OtelProtocol,
			ServiceName:        serviceName,
			ResourceAttributes: opts.OtelResourceAttributes,
			SampleRatio:        opts.OtelSampleRatio,
		})
		if err != nil {
			return nil, err
		}
		shutdown = tpShutdown
	}

	if opts.PyroscopeAddr != "" {
		pyroscopex.InitPyroscope(serviceName, opts.PyroscopeAddr)
	}

	if err := otel.InitMetricProvider(); err != nil {
		return shutdown, err
	}

	return shutdown, nil
}

func ResolveServiceName(override, configServiceName string) string {
	if override != "" {
		return override
	}
	if configServiceName != "" {
		return configServiceName
	}
	return "devflow"
}

func ReinjectLogger(ctx context.Context) context.Context {
	return logger.InjectLogger(ctx, logger.LoggerFromContext(ctx))
}

func StartSpan(ctx context.Context, tracer trace.Tracer, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, spanName, opts...)
	return ReinjectLogger(ctx), span
}

var devflowTracer = gootel.Tracer("devflow")

func StartServiceSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return StartSpan(ctx, devflowTracer, spanName, opts...)
}

func StartWorkerSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return StartSpan(ctx, gootel.Tracer("release-worker"), spanName, opts...)
}
