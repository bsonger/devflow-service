package otel

import (
	"context"
	"sync"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

var (
	buildInfoMetricOnce sync.Once
	buildInfoMetricErr  error
)

func InitMetricProvider() error {
	exporter, err := prometheus.New()
	if err != nil {
		return err
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithExemplarFilter(metricExemplarFilter),
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{
				Name: "http_server_request_duration_seconds",
				Kind: sdkmetric.InstrumentKindHistogram,
				Unit: "s",
			},
			sdkmetric.Stream{
				Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
					Boundaries: []float64{
						0.001,
						0.0025,
						0.005,
						0.01,
						0.025,
						0.05,
						0.1,
						0.25,
						0.5,
						1,
						2.5,
						5,
						10,
					},
				},
			},
		)),
	)
	otel.SetMeterProvider(provider)

	buildInfoMetricOnce.Do(func() {
		buildInfoMetricErr = registerBuildInfoMetric()
	})
	if buildInfoMetricErr != nil {
		return buildInfoMetricErr
	}
	return nil
}

func registerBuildInfoMetric() error {
	meter := otel.Meter("devflow/build")
	buildInfo, err := meter.Int64ObservableGauge(
		"build_info",
		metric.WithUnit("{build}"),
		metric.WithDescription("Build and version metadata for the running service."),
	)
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		observer.ObserveInt64(buildInfo, 1, metric.WithAttributes(
			attribute.String("service_name", logger.ServiceName()),
			attribute.String("service_version", logger.ServiceVersion()),
			attribute.String("git_commit", logger.GitCommit()),
		))
		return nil
	}, buildInfo)
	return err
}
