package otel

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func TestPrometheusExporterServesHTTPLatencyHistogramWithStableHelp(t *testing.T) {
	originalGatherer := prometheus.DefaultGatherer
	originalRegisterer := prometheus.DefaultRegisterer
	registry := prometheus.NewRegistry()
	prometheus.DefaultGatherer = registry
	prometheus.DefaultRegisterer = registry
	t.Cleanup(func() {
		prometheus.DefaultGatherer = originalGatherer
		prometheus.DefaultRegisterer = originalRegisterer
	})

	if err := InitMetricProvider(); err != nil {
		t.Fatalf("InitMetricProvider() error = %v", err)
	}

	histogram, err := otel.Meter("test/http").Float64Histogram(
		"http_server_request_duration_seconds",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of HTTP server requests."),
	)
	if err != nil {
		t.Fatalf("Float64Histogram() error = %v", err)
	}

	histogram.Record(context.Background(), 0.0015, metric.WithAttributes(
		attribute.String("service_name", "config-service"),
		attribute.String("service_namespace", "devflow"),
		attribute.String("deployment_environment_name", "pre-production"),
		attribute.String("http_request_method", "GET"),
		attribute.String("http_route", "/api/v1/workload-configs"),
		attribute.String("http_response_status_code", "200"),
		attribute.String("http_response_status_class", "2xx"),
	))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var helpLine string
	scanner := bufio.NewScanner(strings.NewReader(recorder.Body.String()))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# HELP http_server_request_duration_seconds ") {
			helpLine = line
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error = %v", err)
	}
	if helpLine == "" {
		t.Fatal("missing HELP line for http_server_request_duration_seconds")
	}
	if !strings.Contains(helpLine, "Duration of HTTP server requests.") {
		t.Fatalf("unexpected HELP line: %s", helpLine)
	}
}
