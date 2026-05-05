package routercore

import (
	"testing"
	"time"
)

func TestShouldIgnorePathIncludesLowValueProbePaths(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz", "/livez", "/metrics", "/favicon.ico"} {
		if !ShouldIgnorePath(path) {
			t.Fatalf("ShouldIgnorePath(%q) = false, want true", path)
		}
	}
}

func TestShouldSkipHTTPRequestLogKeepsIncidentSignals(t *testing.T) {
	if !shouldSkipHTTPRequestLog("/healthz", 200, 10*time.Millisecond) {
		t.Fatal("expected fast successful healthz request to be skipped")
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
