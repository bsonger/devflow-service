package service

import (
	"context"
	"strings"
	"sync"
	"time"

	platformotel "github.com/bsonger/devflow-service/internal/platform/otel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	runtimeActionMetricsOnce        sync.Once
	runtimeActionMetricsInitErr     error
	runtimeActionTotalCounter       metric.Int64Counter
	runtimeActionFailedCounter      metric.Int64Counter
	runtimeActionDurationHistogram  metric.Float64Histogram
	observeRuntimeActionMetricsFunc = observeRuntimeActionMetrics
)

func observeRuntimeAction(ctx context.Context, action string, success bool, duration time.Duration) {
	observeRuntimeActionMetricsFunc(ctx, action, success, duration)
}

func observeRuntimeActionMetrics(ctx context.Context, action string, success bool, duration time.Duration) {
	runtimeActionMetricsOnce.Do(initRuntimeActionMetrics)
	if runtimeActionMetricsInitErr != nil {
		return
	}
	action = normalizeRuntimeActionLabel(action)
	if action == "" {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("action", action),
		attribute.String("result", successResultLabel(success)),
	}
	runtimeActionTotalCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	if !success {
		ctx = platformotel.WithFailureMetricExemplarContext(ctx, true)
		runtimeActionFailedCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	runtimeActionDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

func initRuntimeActionMetrics() {
	meter := otel.Meter("devflow/runtime")
	runtimeActionTotalCounter, runtimeActionMetricsInitErr = meter.Int64Counter("runtime_action_total", metric.WithUnit("{action}"))
	if runtimeActionMetricsInitErr != nil {
		return
	}
	runtimeActionFailedCounter, runtimeActionMetricsInitErr = meter.Int64Counter("runtime_action_failed_total", metric.WithUnit("{action}"))
	if runtimeActionMetricsInitErr != nil {
		return
	}
	runtimeActionDurationHistogram, runtimeActionMetricsInitErr = meter.Float64Histogram("runtime_action_duration_seconds", metric.WithUnit("s"))
}

func normalizeRuntimeActionLabel(action string) string {
	switch strings.TrimSpace(action) {
	case "delete_pod", "restart_deployment", "sync_runtime_pod", "sync_runtime_workload":
		return strings.TrimSpace(action)
	default:
		return ""
	}
}

func successResultLabel(success bool) string {
	if success {
		return "success"
	}
	return "error"
}
