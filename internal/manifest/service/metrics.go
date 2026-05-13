package service

import (
	"context"
	"strings"
	"sync"
	"time"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	platformotel "github.com/bsonger/devflow-service/internal/platform/otel"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const manifestPipelineType = "tekton"

var (
	manifestMetricsOnce        sync.Once
	manifestMetricsInitErr     error
	manifestTotalCounter       metric.Int64Counter
	manifestFailedCounter      metric.Int64Counter
	manifestDurationHistogram  metric.Float64Histogram
	observeManifestMetricsFunc = observeManifestMetrics
)

func observeManifest(ctx context.Context, manifest *manifestdomain.Manifest, success bool, duration time.Duration) {
	observeManifestMetricsFunc(ctx, manifest, success, duration)
}

func observeManifestMetrics(ctx context.Context, manifest *manifestdomain.Manifest, success bool, duration time.Duration) {
	manifestMetricsOnce.Do(initManifestMetrics)
	if manifestMetricsInitErr != nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("pipeline_type", manifestPipelineType),
		attribute.String("result", successResultLabel(success)),
	}
	manifestTotalCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	if !success {
		ctx = platformotel.WithFailureMetricExemplarContext(ctx, true)
		manifestFailedCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	manifestDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

func initManifestMetrics() {
	meter := otel.Meter("devflow/manifest")
	manifestTotalCounter, manifestMetricsInitErr = meter.Int64Counter("manifest_total", metric.WithUnit("{manifest}"))
	if manifestMetricsInitErr != nil {
		return
	}
	manifestFailedCounter, manifestMetricsInitErr = meter.Int64Counter("manifest_failed_total", metric.WithUnit("{manifest}"))
	if manifestMetricsInitErr != nil {
		return
	}
	manifestDurationHistogram, manifestMetricsInitErr = meter.Float64Histogram("manifest_duration_seconds", metric.WithUnit("s"))
}

func manifestTerminalDuration(item *manifestdomain.Manifest, fallbackStart time.Time) time.Duration {
	if item != nil && !item.CreatedAt.IsZero() {
		return time.Since(item.CreatedAt)
	}
	if !fallbackStart.IsZero() {
		return time.Since(fallbackStart)
	}
	return 0
}

func normalizeManifestStatusLabel(value model.ManifestStatus) string {
	switch strings.TrimSpace(string(value)) {
	case string(model.ManifestPending), string(model.ManifestRunning), string(model.ManifestAvailable), string(model.ManifestUnavailable):
		return strings.TrimSpace(string(value))
	default:
		return string(model.ManifestPending)
	}
}

func successResultLabel(success bool) string {
	if success {
		return "success"
	}
	return "error"
}
