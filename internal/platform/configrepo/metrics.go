package configrepo

import (
	"context"
	"sync"
	"time"

	platformotel "github.com/bsonger/devflow-service/internal/platform/otel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const configRepoSyncSource = "git_repo"

var (
	configRepoMetricsOnce            sync.Once
	configRepoMetricsInitErr         error
	configRepoSyncTotal              metric.Int64Counter
	configRepoSyncSuccessTotal       metric.Int64Counter
	configRepoSyncFailedTotal        metric.Int64Counter
	configRepoSyncDurationHistogram  metric.Float64Histogram
	observeConfigRepoSyncMetricsFunc = observeConfigRepoSyncMetrics
)

func observeConfigRepoSync(ctx context.Context, success bool, duration time.Duration) {
	observeConfigRepoSyncMetricsFunc(ctx, success, duration)
}

func observeConfigRepoSyncMetrics(ctx context.Context, success bool, duration time.Duration) {
	configRepoMetricsOnce.Do(initConfigRepoMetrics)
	if configRepoMetricsInitErr != nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("sync_source", configRepoSyncSource),
		attribute.String("result", successResultLabel(success)),
	}
	configRepoSyncTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	if success {
		configRepoSyncSuccessTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		ctx = platformotel.WithFailureMetricExemplarContext(ctx, true)
		configRepoSyncFailedTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	configRepoSyncDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

func initConfigRepoMetrics() {
	meter := otel.Meter("devflow/configrepo")
	configRepoSyncTotal, configRepoMetricsInitErr = meter.Int64Counter("app_config_sync_total", metric.WithUnit("{sync}"))
	if configRepoMetricsInitErr != nil {
		return
	}
	configRepoSyncSuccessTotal, configRepoMetricsInitErr = meter.Int64Counter("app_config_sync_success_total", metric.WithUnit("{sync}"))
	if configRepoMetricsInitErr != nil {
		return
	}
	configRepoSyncFailedTotal, configRepoMetricsInitErr = meter.Int64Counter("app_config_sync_failed_total", metric.WithUnit("{sync}"))
	if configRepoMetricsInitErr != nil {
		return
	}
	configRepoSyncDurationHistogram, configRepoMetricsInitErr = meter.Float64Histogram("app_config_sync_duration_seconds", metric.WithUnit("s"))
}

func successResultLabel(success bool) string {
	if success {
		return "success"
	}
	return "error"
}
