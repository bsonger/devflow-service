package downstreamhttp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestGetEnvelopeDataPropagatesRequestIDAndTraceContext(t *testing.T) {
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
			t.Fatal("expected traceparent header to be propagated")
		}
		if got := r.Header.Get("X-Trace-Id"); got == "" {
			t.Fatal("expected X-Trace-Id header to be propagated")
		}
		if got := r.Header.Get("X-Span-Id"); got == "" {
			t.Fatal("expected X-Span-Id header to be propagated")
		}
		_, _ = io.WriteString(w, `{"data":{"id":"app-1"}}`)
	}))
	defer ts.Close()

	ctx := logger.WithRequestID(context.Background(), "req-123")
	ctx, span := otel.Tracer("test").Start(ctx, "root")
	defer span.End()

	var out struct {
		ID string `json:"id"`
	}
	if err := New(ts.URL).GetEnvelopeData(ctx, "/api/v1/applications/app-1", &out); err != nil {
		t.Fatalf("GetEnvelopeData error = %v", err)
	}
	if out.ID != "app-1" {
		t.Fatalf("id = %q, want app-1", out.ID)
	}
}

func TestGetEnvelopeDataReturnsTypedStatusError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer ts.Close()

	var out struct{}
	err := New(ts.URL).GetEnvelopeData(context.Background(), "/api/v1/projects/proj-1", &out)
	if !IsStatus(err, http.StatusNotFound) {
		t.Fatalf("expected IsStatus(..., 404) to be true, err = %v", err)
	}

	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected StatusError, got %T", err)
	}
	if statusErr.StatusCode != http.StatusNotFound {
		t.Fatalf("status code = %d, want 404", statusErr.StatusCode)
	}
	if statusErr.Path != "/api/v1/projects/proj-1" {
		t.Fatalf("path = %q, want /api/v1/projects/proj-1", statusErr.Path)
	}
}

func TestGetEnvelopeDataWithOperationUsesConfiguredDependency(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"id":"env-1"}}`)
	}))
	defer ts.Close()

	var out struct {
		ID string `json:"id"`
	}
	client := NewWithOptions(ts.URL, WithDependency("http", "meta_service"))
	if err := client.GetEnvelopeDataWithOperation(context.Background(), "/api/v1/environments/env-1", "get_environment", &out); err != nil {
		t.Fatalf("GetEnvelopeDataWithOperation error = %v", err)
	}
	if out.ID != "env-1" {
		t.Fatalf("id = %q, want env-1", out.ID)
	}
}
