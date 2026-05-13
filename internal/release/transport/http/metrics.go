package http

import (
	"context"
	"strings"
	"sync"
	"time"

	platformotel "github.com/bsonger/devflow-service/internal/platform/otel"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	releaseWritebackMetricsOnce       sync.Once
	releaseWritebackMetricsInitErr    error
	releaseWritebackTotalCounter      metric.Int64Counter
	releaseWritebackFailedCounter     metric.Int64Counter
	releaseWritebackDurationHistogram metric.Float64Histogram

	argoRolloutTotalCounter      metric.Int64Counter
	argoRolloutSuccessCounter    metric.Int64Counter
	argoRolloutFailedCounter     metric.Int64Counter
	argoRolloutDurationHistogram metric.Float64Histogram

	observeReleaseWritebackMetricsFunc = observeReleaseWritebackMetrics
	observeArgoRolloutMetricsFunc      = observeArgoRolloutMetrics
)

func observeReleaseWriteback(ctx context.Context, callbackType string, success bool, duration time.Duration) {
	observeReleaseWritebackMetricsFunc(ctx, callbackType, success, duration)
}

func observeReleaseWritebackMetrics(ctx context.Context, callbackType string, success bool, duration time.Duration) {
	releaseWritebackMetricsOnce.Do(initReleaseWritebackMetrics)
	if releaseWritebackMetricsInitErr != nil {
		return
	}
	callbackType = normalizeCallbackTypeLabel(callbackType)
	if callbackType == "" {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("callback_type", callbackType),
		attribute.String("result", successResultLabel(success)),
	}
	releaseWritebackTotalCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	if !success {
		ctx = platformotel.WithFailureMetricExemplarContext(ctx, true)
		releaseWritebackFailedCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	releaseWritebackDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

func observeArgoRollout(ctx context.Context, release *model.Release, status model.ReleaseStatus, duration time.Duration) {
	observeArgoRolloutMetricsFunc(ctx, release, status, duration)
}

func observeArgoRolloutMetrics(ctx context.Context, release *model.Release, status model.ReleaseStatus, duration time.Duration) {
	releaseWritebackMetricsOnce.Do(initReleaseWritebackMetrics)
	if releaseWritebackMetricsInitErr != nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("strategy", normalizedReleaseStrategyLabel(release)),
		attribute.String("result", releaseStatusResultLabel(status)),
	}
	argoRolloutTotalCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	switch status {
	case model.ReleaseSucceeded:
		argoRolloutSuccessCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	case model.ReleaseFailed, model.ReleaseSyncFailed:
		ctx = platformotel.WithFailureMetricExemplarContext(ctx, true)
		argoRolloutFailedCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	default:
	}
	if duration > 0 {
		argoRolloutDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	}
}

func initReleaseWritebackMetrics() {
	meter := otel.Meter("devflow/release_writeback")
	releaseWritebackTotalCounter, releaseWritebackMetricsInitErr = meter.Int64Counter("release_writeback_total", metric.WithUnit("{callback}"))
	if releaseWritebackMetricsInitErr != nil {
		return
	}
	releaseWritebackFailedCounter, releaseWritebackMetricsInitErr = meter.Int64Counter("release_writeback_failed_total", metric.WithUnit("{callback}"))
	if releaseWritebackMetricsInitErr != nil {
		return
	}
	releaseWritebackDurationHistogram, releaseWritebackMetricsInitErr = meter.Float64Histogram("release_writeback_duration_seconds", metric.WithUnit("s"))
	if releaseWritebackMetricsInitErr != nil {
		return
	}
	argoRolloutTotalCounter, releaseWritebackMetricsInitErr = meter.Int64Counter("argo_rollout_total", metric.WithUnit("{rollout}"))
	if releaseWritebackMetricsInitErr != nil {
		return
	}
	argoRolloutSuccessCounter, releaseWritebackMetricsInitErr = meter.Int64Counter("argo_rollout_success_total", metric.WithUnit("{rollout}"))
	if releaseWritebackMetricsInitErr != nil {
		return
	}
	argoRolloutFailedCounter, releaseWritebackMetricsInitErr = meter.Int64Counter("argo_rollout_failed_total", metric.WithUnit("{rollout}"))
	if releaseWritebackMetricsInitErr != nil {
		return
	}
	argoRolloutDurationHistogram, releaseWritebackMetricsInitErr = meter.Float64Histogram("argo_rollout_duration_seconds", metric.WithUnit("s"))
}

func normalizeCallbackTypeLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "argo_event", "release_step", "release_artifact":
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func normalizedReleaseStrategyLabel(release *model.Release) string {
	if release == nil {
		return "unknown"
	}
	if value := strings.TrimSpace(release.Strategy); value != "" {
		return value
	}
	return "unknown"
}

func releaseStatusResultLabel(status model.ReleaseStatus) string {
	switch status {
	case model.ReleaseSucceeded:
		return "success"
	case model.ReleaseFailed, model.ReleaseSyncFailed:
		return "error"
	case model.ReleaseRunning:
		return "running"
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
