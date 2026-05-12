package service

import (
	"context"
	"strings"
	"sync"
	"time"

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
	return []attribute.KeyValue{
		attribute.String("release_type", normalizedReleaseTypeLabel(release)),
	}
}

func normalizedReleaseTypeLabel(release *model.Release) string {
	if release == nil {
		return "unknown"
	}
	if value := strings.TrimSpace(release.Type); value != "" {
		return value
	}
	return "unknown"
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

func normalizeReleaseStageLabel(stage string) string {
	switch strings.TrimSpace(stage) {
	case "render_deployment_bundle":
		return "render_bundle"
	case "publish_bundle",
		"create_argocd_application",
		"start_deployment",
		"observe_rollout",
		"finalize_release",
		"deploy_preview",
		"observe_preview",
		"switch_traffic",
		"verify_active",
		"deploy_canary",
		"canary_10",
		"canary_30",
		"canary_60",
		"canary_100":
		return strings.TrimSpace(stage)
	default:
		return ""
	}
}
