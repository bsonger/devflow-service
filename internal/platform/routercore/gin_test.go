package routercore

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

func TestShouldIgnorePathIncludesLowValueProbePaths(t *testing.T) {
	for _, path := range []string{
		"/health",
		"/healthz",
		"/readyz",
		"/livez",
		"/metrics",
		"/favicon.ico",
		"/internal/status",
		"/debug/pprof",
		"/debug/pprof/profile",
		"/swagger",
		"/swagger/index.html",
	} {
		if !ShouldIgnorePath(path) {
			t.Fatalf("ShouldIgnorePath(%q) = false, want true", path)
		}
	}
}

func TestOtelFilterSkipsLowValuePathsWithoutSamplingOtherRoutes(t *testing.T) {
	for _, path := range []string{
		"/health",
		"/healthz",
		"/readyz",
		"/livez",
		"/metrics",
		"/favicon.ico",
		"/internal/status",
		"/debug/pprof/heap",
		"/swagger/doc.json",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if OtelFilter(req) {
			t.Fatalf("OtelFilter(%q) = true, want false", path)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	if !OtelFilter(req) {
		t.Fatal("OtelFilter(/api/v1/projects) = false, want true")
	}
}

func TestShouldSkipHTTPRequestLogKeepsIncidentSignals(t *testing.T) {
	if !shouldSkipHTTPRequestLog("/healthz", 200, 10*time.Millisecond) {
		t.Fatal("expected fast successful healthz request to be skipped")
	}
	if !shouldSkipHTTPRequestLog("/internal/status", 200, 10*time.Millisecond) {
		t.Fatal("expected fast successful internal status request to be skipped")
	}
	if !shouldSkipHTTPRequestLog("/debug/pprof/heap", 200, 10*time.Millisecond) {
		t.Fatal("expected fast successful pprof request to be skipped")
	}
	if !shouldSkipHTTPRequestLog("/swagger/index.html", 200, 10*time.Millisecond) {
		t.Fatal("expected fast successful swagger request to be skipped")
	}
	if shouldSkipHTTPRequestLog("/healthz", 500, 10*time.Millisecond) {
		t.Fatal("expected 5xx healthz request to be logged")
	}
	if shouldSkipHTTPRequestLog("/healthz", 200, 1500*time.Millisecond) {
		t.Fatal("expected slow healthz request to be logged")
	}
	if shouldSkipHTTPRequestLog("/api/v1/projects", 200, 10*time.Millisecond) {
		t.Fatal("expected normal API request to be logged")
	}
}

func TestShouldSkipHTTPMetricMatchesRequestLogIncidentPolicy(t *testing.T) {
	if !shouldSkipHTTPMetric("/metrics", 200, 10*time.Millisecond) {
		t.Fatal("expected fast successful metrics scrape to be skipped")
	}
	if !shouldSkipHTTPMetric("/favicon.ico", 200, 10*time.Millisecond) {
		t.Fatal("expected fast successful favicon request to be skipped")
	}
	if !shouldSkipHTTPMetric("/debug/pprof/profile", 200, 10*time.Millisecond) {
		t.Fatal("expected fast successful pprof request to be skipped")
	}
	if !shouldSkipHTTPMetric("/swagger/doc.json", 200, 10*time.Millisecond) {
		t.Fatal("expected fast successful swagger request to be skipped")
	}
	if shouldSkipHTTPMetric("/metrics", 503, 10*time.Millisecond) {
		t.Fatal("expected 5xx metrics scrape to be recorded")
	}
	if shouldSkipHTTPMetric("/readyz", 200, 1500*time.Millisecond) {
		t.Fatal("expected slow readiness probe to be recorded")
	}
}

func TestHTTPStatusClassUsesInFlightForUnsetStatus(t *testing.T) {
	if got := httpStatusClass(0); got != "in_flight" {
		t.Fatalf("httpStatusClass(0) = %q, want in_flight", got)
	}
}

func TestRequestBodySizeFieldOmitsEmptyGetAndHeadBodies(t *testing.T) {
	if _, ok := requestBodySizeField(http.MethodGet, 0); ok {
		t.Fatal("expected empty GET body size to be omitted")
	}
	if _, ok := requestBodySizeField(http.MethodHead, 0); ok {
		t.Fatal("expected empty HEAD body size to be omitted")
	}
	if field, ok := requestBodySizeField(http.MethodPost, 0); !ok || field.Key != "http.request.body.size" {
		t.Fatalf("expected POST field, got %#v ok=%v", field, ok)
	}
}

func TestShouldIncludeDevflowAccessFields(t *testing.T) {
	if shouldIncludeDevflowAccessFields(200, 10*time.Millisecond) {
		t.Fatal("expected normal 2xx request to omit devflow identifiers")
	}
	if !shouldIncludeDevflowAccessFields(404, 10*time.Millisecond) {
		t.Fatal("expected 4xx request to keep devflow identifiers")
	}
	if !shouldIncludeDevflowAccessFields(200, 1500*time.Millisecond) {
		t.Fatal("expected slow 2xx request to keep devflow identifiers")
	}
}

func TestHTTPRequestLoggerNames(t *testing.T) {
	if got := httpRequestLogger(nil, 200).Name(); got != "http.access" {
		t.Fatalf("httpRequestLogger(200).Name() = %q", got)
	}
	if got := httpRequestLogger(nil, 500).Name(); got != "http.error" {
		t.Fatalf("httpRequestLogger(500).Name() = %q", got)
	}
	if got := httpRequestLogger(nil, 404).Name(); got != "http.error" {
		t.Fatalf("httpRequestLogger(404).Name() = %q", got)
	}
}

func TestHTTPRequestMessage(t *testing.T) {
	if got := httpRequestMessage(200, 10*time.Millisecond); got != "http request" {
		t.Fatalf("httpRequestMessage(200) = %q", got)
	}
	if got := httpRequestMessage(404, 10*time.Millisecond); got != "http client error" {
		t.Fatalf("httpRequestMessage(404) = %q", got)
	}
	if got := httpRequestMessage(500, 10*time.Millisecond); got != "http server error" {
		t.Fatalf("httpRequestMessage(500) = %q", got)
	}
	if got := httpRequestMessage(200, 1500*time.Millisecond); got != "slow http request" {
		t.Fatalf("httpRequestMessage(slow 200) = %q", got)
	}
}

func TestRequestBodySizeFieldReturnsRealZapField(t *testing.T) {
	field, ok := requestBodySizeField(http.MethodPost, 12)
	if !ok {
		t.Fatal("expected body size field")
	}
	if field.Key != "http.request.body.size" {
		t.Fatalf("field.Key = %q", field.Key)
	}
	if field.Type == zapcore.SkipType {
		t.Fatalf("unexpected skip field: %#v", field)
	}
}
