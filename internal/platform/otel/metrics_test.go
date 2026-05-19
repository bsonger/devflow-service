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
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("OTEL_SERVICE_NAMESPACE", "")
	t.Setenv("SERVICE_VERSION", "")
	t.Setenv("VERSION", "")
	t.Setenv("GIT_COMMIT", "")
	t.Setenv("VCS_REVISION", "")
	t.Setenv("COMMIT_HASH", "")
	t.Setenv("DEPLOYMENT_ENVIRONMENT", "")
	t.Setenv("ENVIRONMENT", "")
	t.Setenv("ENV", "")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.name=runtime-service,service.namespace=devflow,service.version=sha256:8fc33fd48da9be177f5d75bf55ed6a6a39cd0a99d841a7604f5e897e50031f52,deployment.environment.name=pre-production")
	t.Setenv("GIT_COMMIT", "1234567890abcdef1234567890abcdef12345678")

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

	body := recorder.Body.String()
	if !strings.Contains(body, "# HELP build_info Build and version metadata for the running service.") {
		t.Fatalf("missing build_info HELP line: %s", body)
	}
	if !strings.Contains(body, "# TYPE build_info gauge") {
		t.Fatalf("missing build_info TYPE line: %s", body)
	}
	if !strings.Contains(body, "service_name=\"runtime-service\"") {
		t.Fatalf("missing service_name label in build_info metric: %s", body)
	}
	if !strings.Contains(body, "service_version=\"sha256:8fc33fd48da9\"") {
		t.Fatalf("missing normalized service_version label in build_info metric: %s", body)
	}
	if !strings.Contains(body, "git_commit=\"1234567890ab\"") {
		t.Fatalf("missing normalized git_commit label in build_info metric: %s", body)
	}
	buildInfoLine := ""
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "build_info{") {
			buildInfoLine = line
			break
		}
	}
	if buildInfoLine == "" {
		t.Fatalf("missing build_info sample line: %s", body)
	}
	if !strings.Contains(buildInfoLine, "otel_scope_name=\"devflow/build\"") {
		t.Fatalf("missing otel_scope_name label in build_info metric: %s", body)
	}
	if strings.Contains(buildInfoLine, "service_namespace=") {
		t.Fatalf("did not expect service_namespace label in build_info metric: %s", body)
	}
	if strings.Contains(buildInfoLine, "deployment_environment=") {
		t.Fatalf("did not expect deployment_environment label in build_info metric: %s", body)
	}
	if strings.Contains(buildInfoLine, "container_image_digest=") {
		t.Fatalf("did not expect container_image_digest label in build_info metric: %s", body)
	}
}
