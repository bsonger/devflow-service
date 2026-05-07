package downstream

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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
	client := &http.Client{
		Timeout: defaultMetricsProbeTimeout,
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
				return fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path)
			}),
		),
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metrics probe returned status %s", resp.Status)
	}
	return nil
}
