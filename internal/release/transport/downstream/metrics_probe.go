package downstream

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

const defaultMetricsProbeTimeout = 3 * time.Second

func ProbeMetricsEndpoint(ctx context.Context, url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("metrics probe url is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if requestID := logger.RequestIDFromContext(ctx); requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
		req.Header.Set("X-Request-ID", requestID)
	}
	if span := trace.SpanFromContext(ctx); span != nil {
		if sc := span.SpanContext(); sc.IsValid() {
			req.Header.Set("X-Trace-Id", sc.TraceID().String())
			req.Header.Set("X-Span-Id", sc.SpanID().String())
		}
	}
	client := &http.Client{
		Timeout: defaultMetricsProbeTimeout,
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
				return fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path)
			}),
		),
	}
	return platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "http",
		Target:    "metrics_endpoint",
		Operation: "probe_metrics_endpoint",
	}, func(depCtx context.Context) error {
		req = req.WithContext(depCtx)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("metrics probe returned status %s", resp.Status)
		}
		return nil
	})
}
