package service

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
	releaseMetricsOnce       sync.Once
	releaseMetricsInitErr    error
	releaseTotalCounter      metric.Int64Counter
	releaseSuccessCounter    metric.Int64Counter
	releaseFailedCounter     metric.Int64Counter
	releaseRollbackCounter   metric.Int64Counter
	releaseDurationHistogram metric.Float64Histogram

	releaseStageTotalCounter      metric.Int64Counter
	releaseStageFailedCounter     metric.Int64Counter
	releaseStageDurationHistogram metric.Float64Histogram

	argoApplicationCreateTotalCounter      metric.Int64Counter
	argoApplicationCreateSuccessCounter    metric.Int64Counter
	argoApplicationCreateFailedCounter     metric.Int64Counter
	argoApplicationCreateDurationHistogram metric.Float64Histogram

	observeReleaseStageMetricsFunc          = observeReleaseStageMetrics
	observeArgoApplicationCreateMetricsFunc = observeArgoApplicationCreateMetrics
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
		ctx = platformotel.WithFailureMetricExemplarContext(ctx, true)
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

func observeReleaseStage(ctx context.Context, release *model.Release, stage string, success bool, duration time.Duration) {
	observeReleaseStageMetricsFunc(ctx, release, stage, success, duration)
}

func observeReleaseStageMetrics(ctx context.Context, release *model.Release, stage string, success bool, duration time.Duration) {
	releaseMetricsOnce.Do(initReleaseMetrics)
	if releaseMetricsInitErr != nil || release == nil {
		return
	}
	stage = normalizeReleaseStageLabel(stage)
	if stage == "" {
		return
	}
	attrs := append(releaseMetricAttributes(release),
		attribute.String("stage", stage),
		attribute.String("result", successResultLabel(success)),
	)
	releaseStageTotalCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	if !success {
		ctx = platformotel.WithFailureMetricExemplarContext(ctx, true)
		releaseStageFailedCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	releaseStageDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

func observeArgoApplicationCreate(ctx context.Context, release *model.Release, success bool, duration time.Duration) {
	observeArgoApplicationCreateMetricsFunc(ctx, release, success, duration)
}

func observeArgoApplicationCreateMetrics(ctx context.Context, release *model.Release, success bool, duration time.Duration) {
	releaseMetricsOnce.Do(initReleaseMetrics)
	if releaseMetricsInitErr != nil || release == nil {
		return
	}
	attrs := append(releaseMetricAttributes(release),
		attribute.String("strategy", normalizedReleaseStrategyLabel(release)),
		attribute.String("result", successResultLabel(success)),
	)
	argoApplicationCreateTotalCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	if success {
		argoApplicationCreateSuccessCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		ctx = platformotel.WithFailureMetricExemplarContext(ctx, true)
		argoApplicationCreateFailedCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	argoApplicationCreateDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
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
	if releaseMetricsInitErr != nil {
		return
	}

	releaseStageTotalCounter, releaseMetricsInitErr = meter.Int64Counter("release_stage_total", metric.WithUnit("{stage}"))
	if releaseMetricsInitErr != nil {
		return
	}
	releaseStageFailedCounter, releaseMetricsInitErr = meter.Int64Counter("release_stage_failed_total", metric.WithUnit("{stage}"))
	if releaseMetricsInitErr != nil {
		return
	}
	releaseStageDurationHistogram, releaseMetricsInitErr = meter.Float64Histogram("release_stage_duration_seconds", metric.WithUnit("s"))
	if releaseMetricsInitErr != nil {
		return
	}

	argoApplicationCreateTotalCounter, releaseMetricsInitErr = meter.Int64Counter("argo_application_create_total", metric.WithUnit("{create}"))
	if releaseMetricsInitErr != nil {
		return
	}
	argoApplicationCreateSuccessCounter, releaseMetricsInitErr = meter.Int64Counter("argo_application_create_success_total", metric.WithUnit("{create}"))
	if releaseMetricsInitErr != nil {
		return
	}
	argoApplicationCreateFailedCounter, releaseMetricsInitErr = meter.Int64Counter("argo_application_create_failed_total", metric.WithUnit("{create}"))
	if releaseMetricsInitErr != nil {
		return
	}
	argoApplicationCreateDurationHistogram, releaseMetricsInitErr = meter.Float64Histogram("argo_application_create_duration_seconds", metric.WithUnit("s"))
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

func successResultLabel(success bool) string {
	if success {
		return "success"
	}
	return "error"
}
