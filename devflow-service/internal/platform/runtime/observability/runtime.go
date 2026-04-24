package observability

import (
	"context"
	"os"

	loggingx "github.com/bsonger/devflow-service/internal/platform/logger"
	otelx "github.com/bsonger/devflow-service/internal/platform/otel"
	pyroscopex "github.com/bsonger/devflow-service/internal/platform/runtime/pyroscopex"
	"go.opentelemetry.io/otel/trace"
)

type RuntimeOptions struct {
	LogLevel        string
	LogFormat       string
	OtelEndpoint    string
	OtelService     string
	OtelSampleRatio float64
	PyroscopeAddr   string
	ServiceOverride string
}

func Init(ctx context.Context, opts RuntimeOptions) (func(context.Context) error, error) {
	loggingx.InitZapLogger(&loggingx.Config{
		Level:  opts.LogLevel,
		Format: opts.LogFormat,
	})

	serviceName := ResolveServiceName(opts.ServiceOverride, opts.OtelService)
	if serviceName != "" {
		_ = os.Setenv("SERVICE_NAME", serviceName)
	}

	shutdown := func(context.Context) error { return nil }
	if opts.OtelEndpoint != "" {
		tpShutdown, err := otelx.InitOtel(ctx, &otelx.Config{
			Endpoint:    opts.OtelEndpoint,
			ServiceName: serviceName,
			SampleRatio: opts.OtelSampleRatio,
		})
		if err != nil {
			return nil, err
		}
		shutdown = tpShutdown
	}

	if opts.PyroscopeAddr != "" {
		pyroscopex.InitPyroscope(serviceName, opts.PyroscopeAddr)
	}

	if err := otelx.InitMetricProvider(); err != nil {
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
	return loggingx.InjectLogger(ctx, loggingx.LoggerFromContext(ctx))
}

func StartSpan(ctx context.Context, tracer trace.Tracer, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, spanName, opts...)
	return ReinjectLogger(ctx), span
}
