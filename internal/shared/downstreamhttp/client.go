package downstreamhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const defaultTimeout = 10 * time.Second

var ErrServiceUnavailable = errors.New("downstream service is not configured")

type StatusError struct {
	Method     string
	Path       string
	StatusCode int
	Status     string
}

type Option func(*config)

type config struct {
	timeout time.Duration
}

func WithTimeout(timeout time.Duration) Option {
	return func(cfg *config) {
		if timeout > 0 {
			cfg.timeout = timeout
		}
	}
}

func (e *StatusError) Error() string {
	if e == nil {
		return "downstream request failed"
	}
	if e.Method != "" || e.Path != "" {
		return fmt.Sprintf("downstream request failed: method=%s path=%s status=%s", e.Method, e.Path, e.Status)
	}
	if e.Status != "" {
		return fmt.Sprintf("downstream request failed: %s", e.Status)
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("downstream request failed: %d", e.StatusCode)
	}
	return "downstream request failed"
}

func (e *StatusError) Is(target error) bool {
	other, ok := target.(*StatusError)
	if !ok {
		return false
	}
	return other.StatusCode == 0 || e.StatusCode == other.StatusCode
}

func Status(statusCode int) error {
	return &StatusError{StatusCode: statusCode}
}

func IsStatus(err error, statusCode int) bool {
	return errors.Is(err, Status(statusCode))
}

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return NewWithOptions(baseURL)
}

func NewWithOptions(baseURL string, opts ...Option) *Client {
	cfg := config{timeout: defaultTimeout}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		http: &http.Client{
			Timeout: cfg.timeout,
			Transport: otelhttp.NewTransport(
				http.DefaultTransport,
				otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
					return fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path)
				}),
			),
		},
	}
}

func (c *Client) GetEnvelopeData(ctx context.Context, path string, out any) error {
	if c == nil || c.baseURL == "" {
		return ErrServiceUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if requestID := logger.RequestIDFromContext(ctx); requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
		req.Header.Set("X-Request-ID", requestID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &StatusError{
			Method:     req.Method,
			Path:       req.URL.Path,
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var envelope map[string]json.RawMessage
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&envelope); err != nil {
		return err
	}
	data, ok := envelope["data"]
	if !ok {
		return json.Unmarshal(body, out)
	}
	return json.Unmarshal(data, out)
}
