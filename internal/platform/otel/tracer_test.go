package otel

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestNewSamplerAlwaysSamplesInApplication(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER", "always_off")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0")

	decision := newSampler(&Config{SampleRatio: 0}).ShouldSample(sdktrace.SamplingParameters{
		ParentContext: context.Background(),
		TraceID:       trace.TraceID{1},
		Name:          "GET /api/v1/projects",
	})
	if decision.Decision != sdktrace.RecordAndSample {
		t.Fatalf("sampler decision = %v, want RecordAndSample", decision.Decision)
	}
}
