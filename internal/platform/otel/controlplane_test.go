package otel

import (
	"context"
	"testing"
)

func TestWithFailureMetricExemplarContext(t *testing.T) {
	ctx := WithFailureMetricExemplarContext(context.Background(), true)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if !metricExemplarFilter(ctx) {
		t.Fatal("expected failed context to force exemplar")
	}

	plain := WithFailureMetricExemplarContext(nil, false)
	if plain == nil {
		t.Fatal("expected background context for nil input")
	}
}

func TestAppendBoundedLabel(t *testing.T) {
	attrs := AppendBoundedLabel(nil, "stage", "", "render_bundle")
	if len(attrs) != 1 {
		t.Fatalf("len(attrs) = %d, want 1", len(attrs))
	}
	if string(attrs[0].Key) != "stage" || attrs[0].Value.AsString() != "render_bundle" {
		t.Fatalf("attrs[0] = %v", attrs[0])
	}

	attrs = AppendBoundedLabel(attrs, "", "ignored", "")
	if len(attrs) != 1 {
		t.Fatalf("blank key changed attrs: %v", attrs)
	}
}
