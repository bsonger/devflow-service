package observer

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
	observerMetricsOnce       sync.Once
	observerMetricsInitErr    error
	runtimeObserverSyncTotal  metric.Int64Counter
	runtimeObserverSyncFailed metric.Int64Counter
	runtimeObserverSyncDur    metric.Float64Histogram
	manifestTaskTotal         metric.Int64Counter
	manifestTaskFailed        metric.Int64Counter
	manifestTaskDur           metric.Float64Histogram

	observeRuntimeObserverSyncMetricsFunc = observeRuntimeObserverSyncMetrics
	observeManifestTaskMetricsFunc        = observeManifestTaskMetrics
)

func observeRuntimeObserverSync(ctx context.Context, observerType string, success bool, duration time.Duration) {
	observeRuntimeObserverSyncMetricsFunc(ctx, observerType, success, duration)
}

func observeRuntimeObserverSyncMetrics(ctx context.Context, observerType string, success bool, duration time.Duration) {
	observerMetricsOnce.Do(initObserverMetrics)
	if observerMetricsInitErr != nil {
		return
	}
	observerType = normalizeObserverTypeLabel(observerType)
	if observerType == "" {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("observer_type", observerType),
		attribute.String("result", successResultLabel(success)),
	}
	runtimeObserverSyncTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	if !success {
		ctx = platformotel.WithFailureMetricExemplarContext(ctx, true)
		runtimeObserverSyncFailed.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	runtimeObserverSyncDur.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

func observeManifestTask(ctx context.Context, taskName, result string, success bool, duration time.Duration) {
	observeManifestTaskMetricsFunc(ctx, taskName, result, success, duration)
}

func observeManifestTaskMetrics(ctx context.Context, taskName, result string, success bool, duration time.Duration) {
	observerMetricsOnce.Do(initObserverMetrics)
	if observerMetricsInitErr != nil {
		return
	}
	taskName = normalizeTaskNameLabel(taskName)
	result = normalizeTaskResultLabel(result)
	if taskName == "" || result == "" {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("task_name", taskName),
		attribute.String("result", result),
	}
	manifestTaskTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	if !success {
		ctx = platformotel.WithFailureMetricExemplarContext(ctx, true)
		manifestTaskFailed.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	manifestTaskDur.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

func initObserverMetrics() {
	meter := otel.Meter("devflow/runtime_observer")
	runtimeObserverSyncTotal, observerMetricsInitErr = meter.Int64Counter("runtime_observer_sync_total", metric.WithUnit("{sync}"))
	if observerMetricsInitErr != nil {
		return
	}
	runtimeObserverSyncFailed, observerMetricsInitErr = meter.Int64Counter("runtime_observer_sync_failed_total", metric.WithUnit("{sync}"))
	if observerMetricsInitErr != nil {
		return
	}
	runtimeObserverSyncDur, observerMetricsInitErr = meter.Float64Histogram("runtime_observer_sync_duration_seconds", metric.WithUnit("s"))
	if observerMetricsInitErr != nil {
		return
	}
	manifestTaskTotal, observerMetricsInitErr = meter.Int64Counter("manifest_task_total", metric.WithUnit("{task}"))
	if observerMetricsInitErr != nil {
		return
	}
	manifestTaskFailed, observerMetricsInitErr = meter.Int64Counter("manifest_task_failed_total", metric.WithUnit("{task}"))
	if observerMetricsInitErr != nil {
		return
	}
	manifestTaskDur, observerMetricsInitErr = meter.Float64Histogram("manifest_task_duration_seconds", metric.WithUnit("s"))
}

func normalizeObserverTypeLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "kubernetes_runtime", "tekton_manifest", "release_rollout":
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func normalizeTaskNameLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	value = strings.ReplaceAll(value, " ", "_")
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func normalizeTaskResultLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "pending", "running", "succeeded", "failed":
		return value
	default:
		return "unknown"
	}
}

func successResultLabel(success bool) string {
	if success {
		return "success"
	}
	return "error"
}
