package observability

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/bsonger/devflow-service/shared/loggingx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

var dependencyTracer = otel.Tracer("devflow/dependency")

var (
	dependencyMetricsOnce    sync.Once
	dependencyCalls          metric.Int64Counter
	dependencyErrors         metric.Int64Counter
	dependencyLatency        metric.Float64Histogram
	dependencyMetricsInitErr error
)

type DependencyCall struct {
	Kind      string
	Target    string
	Operation string
}

func ObserveDependency(ctx context.Context, call DependencyCall, fn func(context.Context) error) error {
	dependencyMetricsOnce.Do(initDependencyMetrics)

	ctx, span := StartSpan(ctx, dependencyTracer, dependencySpanName(call))
	attrs := dependencyAttributes(call)
	for _, attr := range attrs {
		span.SetAttributes(attr)
	}

	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start).Seconds()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		loggingx.LoggerFromContext(ctx).Error("dependency call failed",
			zap.String("component", "dependency_client"),
			zap.String("dependency", call.Target),
			zap.String("result", "error"),
			zap.String("dependency.kind", call.Kind),
			zap.String("dependency.operation", call.Operation),
			zap.Float64("dependency.duration_seconds", duration),
			zap.Error(err),
		)
	} else {
		span.SetStatus(codes.Ok, "ok")
		loggingx.LoggerFromContext(ctx).Info("dependency call completed",
			zap.String("component", "dependency_client"),
			zap.String("dependency", call.Target),
			zap.String("result", "ok"),
			zap.String("dependency.kind", call.Kind),
			zap.String("dependency.operation", call.Operation),
			zap.Float64("dependency.duration_seconds", duration),
		)
	}

	if dependencyMetricsInitErr == nil {
		status := "ok"
		if err != nil {
			status = "error"
		}
		metricAttrs := append(attrs, attribute.String("result", status))
		dependencyCalls.Add(ctx, 1, metric.WithAttributes(metricAttrs...))
		dependencyLatency.Record(ctx, duration, metric.WithAttributes(metricAttrs...))
		if err != nil {
			dependencyErrors.Add(ctx, 1, metric.WithAttributes(metricAttrs...))
		}
	}

	span.End()
	return err
}

func initDependencyMetrics() {
	meter := otel.Meter("devflow/dependency")

	dependencyCalls, dependencyMetricsInitErr = meter.Int64Counter(
		"devflow_dependency_calls_total",
		metric.WithUnit("{request}"),
	)
	if dependencyMetricsInitErr != nil {
		return
	}
	dependencyErrors, dependencyMetricsInitErr = meter.Int64Counter(
		"devflow_dependency_errors_total",
		metric.WithUnit("{error}"),
	)
	if dependencyMetricsInitErr != nil {
		return
	}

	dependencyLatency, dependencyMetricsInitErr = meter.Float64Histogram(
		"devflow_dependency_duration_seconds",
		metric.WithUnit("s"),
	)
}

func dependencyAttributes(call DependencyCall) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("service", serviceName()),
		attribute.String("dependency", safeDependency(call.Target)),
		attribute.String("action", safeAction(call.Operation)),
		attribute.String("devflow.dependency", safeDependency(call.Target)),
		attribute.String("devflow.action", safeAction(call.Operation)),
	}
}

func dependencySpanName(call DependencyCall) string {
	name := call.Target
	if name == "" {
		name = "unknown"
	}
	if call.Operation != "" {
		return name + "." + call.Operation
	}
	return name
}

func serviceName() string {
	if v := os.Getenv("SERVICE_NAME"); v != "" {
		return v
	}
	return "devflow"
}

func safeDependency(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func safeAction(value string) string {
	if value == "" {
		return "call"
	}
	return value
}
