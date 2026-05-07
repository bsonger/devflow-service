package routercore

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestHTTPEventOutcome(t *testing.T) {
	if got := httpEventOutcome(200); got != "success" {
		t.Fatalf("httpEventOutcome(200) = %q, want success", got)
	}
	if got := httpEventOutcome(404); got != "failure" {
		t.Fatalf("httpEventOutcome(404) = %q, want failure", got)
	}
}

func TestHTTPStatusClassUsesInFlightForUnsetStatus(t *testing.T) {
	if got := httpStatusClass(0); got != "in_flight" {
		t.Fatalf("httpStatusClass(0) = %q, want in_flight", got)
	}
}
