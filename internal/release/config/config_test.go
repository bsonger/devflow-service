package config

import (
	"testing"

	model "github.com/bsonger/devflow-service/internal/release/domain"
)

func TestResolveObservabilityServiceName(t *testing.T) {
	cfg := &model.OtelConfig{ServiceName: "otel-service"}

	if got := resolveObservabilityServiceName(cfg, "runtime-override"); got != "runtime-override" {
		t.Fatalf("got %q want runtime-override", got)
	}
	if got := resolveObservabilityServiceName(cfg, ""); got != "otel-service" {
		t.Fatalf("got %q want otel-service", got)
	}
	if got := resolveObservabilityServiceName(nil, ""); got != "devflow" {
		t.Fatalf("got %q want devflow", got)
	}
}
