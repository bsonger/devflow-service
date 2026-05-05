package otel

import (
	"context"

	"go.opentelemetry.io/otel/sdk/metric/exemplar"
)

type metricExemplarContextKey struct{}

func WithMetricExemplar(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, metricExemplarContextKey{}, true)
}

func metricExemplarFilter(ctx context.Context) bool {
	if enabled, _ := ctx.Value(metricExemplarContextKey{}).(bool); enabled {
		return true
	}
	return exemplar.TraceBasedFilter(ctx)
}
