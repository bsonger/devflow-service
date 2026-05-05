package otel

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestMetricExemplarFilterUsesTraceBasedDefault(t *testing.T) {
	if metricExemplarFilter(context.Background()) {
		t.Fatal("background context should not be offered to the exemplar reservoir")
	}

	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	}))
	if !metricExemplarFilter(ctx) {
		t.Fatal("sampled trace context should be offered to the exemplar reservoir")
	}
}

func TestMetricExemplarFilterCanForceIncidentExemplars(t *testing.T) {
	if !metricExemplarFilter(WithMetricExemplar(context.Background())) {
		t.Fatal("forced exemplar context should be offered to the exemplar reservoir")
	}
}
