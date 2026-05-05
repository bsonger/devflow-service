package otel

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
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
	return nil
}
