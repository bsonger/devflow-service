package downstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestProbeMetricsEndpointPropagatesTraceAndRequestHeaders(t *testing.T) {
	originalTP := otel.GetTracerProvider()
	originalPropagator := otel.GetTextMapPropagator()
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer func() {
		otel.SetTracerProvider(originalTP)
		otel.SetTextMapPropagator(originalPropagator)
		_ = tp.Shutdown(context.Background())
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Request-Id"); got != "req-123" {
			t.Fatalf("X-Request-Id = %q, want req-123", got)
		}
		if got := r.Header.Get("traceparent"); got == "" {
			t.Fatal("expected traceparent header")
		}
		if got := r.Header.Get("X-Trace-Id"); got == "" {
			t.Fatal("expected X-Trace-Id header")
		}
		if got := r.Header.Get("X-Span-Id"); got == "" {
			t.Fatal("expected X-Span-Id header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	ctx := logger.WithRequestID(context.Background(), "req-123")
	ctx, span := otel.Tracer("test").Start(ctx, "root")
	defer span.End()

	if err := ProbeMetricsEndpoint(ctx, ts.URL); err != nil {
		t.Fatalf("ProbeMetricsEndpoint error = %v", err)
	}
}
