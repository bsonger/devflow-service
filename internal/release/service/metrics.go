package service

import (
	"context"
	"sync"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	releaseMetricsOnce       sync.Once
	releaseMetricsInitErr    error
	releaseTotalCounter      metric.Int64Counter
	releaseSuccessCounter    metric.Int64Counter
	releaseFailedCounter     metric.Int64Counter
	releaseRollbackCounter   metric.Int64Counter
	releaseDurationHistogram metric.Float64Histogram
)

func observeReleaseCreated(ctx context.Context, release *model.Release) {
	releaseMetricsOnce.Do(initReleaseMetrics)
	if releaseMetricsInitErr != nil || release == nil {
		return
	}
	releaseTotalCounter.Add(ctx, 1, metric.WithAttributes(releaseMetricAttributes(release)...))
}

func observeReleaseTerminal(ctx context.Context, release *model.Release, status model.ReleaseStatus) {
	releaseMetricsOnce.Do(initReleaseMetrics)
	if releaseMetricsInitErr != nil || release == nil {
		return
	}

	attrs := releaseMetricAttributes(release)
	switch status {
	case model.ReleaseSucceeded:
		releaseSuccessCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	case model.ReleaseFailed, model.ReleaseSyncFailed:
		releaseFailedCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	case model.ReleaseRolledBack:
		releaseRollbackCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	default:
		return
	}

	if !release.CreatedAt.IsZero() {
		releaseDurationHistogram.Record(ctx, time.Since(release.CreatedAt).Seconds(), metric.WithAttributes(attrs...))
	}
}

func initReleaseMetrics() {
	meter := otel.Meter("devflow/release")

	releaseTotalCounter, releaseMetricsInitErr = meter.Int64Counter("release_total", metric.WithUnit("{release}"))
	if releaseMetricsInitErr != nil {
		return
	}
	releaseSuccessCounter, releaseMetricsInitErr = meter.Int64Counter("release_success_total", metric.WithUnit("{release}"))
	if releaseMetricsInitErr != nil {
		return
	}
	releaseFailedCounter, releaseMetricsInitErr = meter.Int64Counter("release_failed_total", metric.WithUnit("{release}"))
	if releaseMetricsInitErr != nil {
		return
	}
	releaseRollbackCounter, releaseMetricsInitErr = meter.Int64Counter("deployment_rollback_total", metric.WithUnit("{release}"))
	if releaseMetricsInitErr != nil {
		return
	}
	releaseDurationHistogram, releaseMetricsInitErr = meter.Float64Histogram("release_duration_seconds", metric.WithUnit("s"))
}

func releaseMetricAttributes(release *model.Release) []attribute.KeyValue {
	releaseType := "unknown"
	if release != nil && release.Type != "" {
		releaseType = release.Type
	}
	return []attribute.KeyValue{
		attribute.String("service", logger.ServiceName()),
		attribute.String("environment", logger.Environment()),
		attribute.String("release_type", releaseType),
	}
}
