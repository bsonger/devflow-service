package routercore

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/grafana/pyroscope-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func ShouldIgnorePath(path string) bool {
	return path == "/metrics" ||
		path == "/health" ||
		path == "/healthz" ||
		path == "/readyz" ||
		path == "/internal/status" ||
		strings.HasPrefix(path, "/debug/pprof") ||
		strings.HasPrefix(path, "/swagger")
}

func OtelFilter(req *http.Request) bool {
	return !ShouldIgnorePath(req.URL.Path)
}

func routeLabel(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	return "unknown"
}

func PyroscopeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		method := c.Request.Method
		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}

		pyroscope.TagWrapper(ctx, pyroscope.Labels("http.route", route, "http.method", method), func(ctx context.Context) {
			c.Next()
		})
	}
}

var (
	httpMetricsOnce      sync.Once
	httpRequestsCounter  metric.Int64Counter
	httpRequestsInFlight metric.Int64UpDownCounter
	httpRequestLatency   metric.Float64Histogram
	httpRequestSize      metric.Int64Histogram
	httpResponseSize     metric.Int64Histogram
	httpMetricsInitErr   error
)

func GinMetricsMiddleware() gin.HandlerFunc {
	httpMetricsOnce.Do(initHTTPMetrics)

	return func(c *gin.Context) {
		if ShouldIgnorePath(c.Request.URL.Path) {
			c.Next()
			return
		}

		start := time.Now()
		requestSize := c.Request.ContentLength
		if requestSize < 0 {
			requestSize = 0
		}
		ctx := c.Request.Context()
		startAttrs := httpMetricAttributes(c, 0)
		if httpMetricsInitErr == nil {
			httpRequestsInFlight.Add(ctx, 1, metric.WithAttributes(startAttrs...))
			defer httpRequestsInFlight.Add(ctx, -1, metric.WithAttributes(startAttrs...))
		}

		c.Next()

		if httpMetricsInitErr != nil {
			return
		}

		status := c.Writer.Status()
		attrs := httpMetricAttributes(c, status)
		httpRequestsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		httpRequestLatency.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		httpRequestSize.Record(ctx, requestSize, metric.WithAttributes(attrs...))
		httpResponseSize.Record(ctx, int64(maxInt(c.Writer.Size(), 0)), metric.WithAttributes(attrs...))
	}
}

func initHTTPMetrics() {
	meter := otel.Meter("devflow/http")

	httpRequestsCounter, httpMetricsInitErr = meter.Int64Counter(
		"http_server_requests_total",
		metric.WithUnit("{request}"),
	)
	if httpMetricsInitErr != nil {
		return
	}
	httpRequestsInFlight, httpMetricsInitErr = meter.Int64UpDownCounter(
		"http_server_requests_in_flight",
		metric.WithUnit("{request}"),
	)
	if httpMetricsInitErr != nil {
		return
	}

	httpRequestLatency, httpMetricsInitErr = meter.Float64Histogram(
		"http_server_request_duration_seconds",
		metric.WithUnit("s"),
	)
	if httpMetricsInitErr != nil {
		return
	}

	httpRequestSize, httpMetricsInitErr = meter.Int64Histogram(
		"http_server_request_size_bytes",
		metric.WithUnit("By"),
	)
	if httpMetricsInitErr != nil {
		return
	}

	httpResponseSize, httpMetricsInitErr = meter.Int64Histogram(
		"http_server_response_size_bytes",
		metric.WithUnit("By"),
	)
}

func GinZapLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		req := c.Request
		path := req.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}

		fields := []zap.Field{
			zap.String("component", "http_server"),
			zap.String("result", httpResult(status)),
			zap.String("method", req.Method),
			zap.String("route", route),
			zap.String("path", path),
			zap.Int("status_code", status),
			zap.Int64("request_size_bytes", maxInt64(req.ContentLength, 0)),
			zap.Int("response_size_bytes", maxInt(c.Writer.Size(), 0)),
			zap.Int64("duration_ms", latency.Milliseconds()),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", req.UserAgent()),
		}

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			fields = append(fields, zap.String("error_message", err.Error()))
		}

		log := logger.LoggerFromContext(req.Context())

		switch {
		case status >= 500:
			log.Error("http request", fields...)
		case status >= 400:
			log.Warn("http request", fields...)
		case latency >= time.Second:
			log.Warn("slow http request", fields...)
		default:
			log.Info("http request", fields...)
		}
	}
}

func GinZapRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log := logger.LoggerFromContext(c.Request.Context())
				log.Error("panic recovered",
					zap.String("component", "http_server"),
					zap.String("result", "panic"),
					zap.Any("panic", rec),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.String("client_ip", c.ClientIP()),
				)
				httpx.WriteError(c, http.StatusInternalServerError, "internal", "internal server error", nil)
				c.Abort()
			}
		}()
		c.Next()
	}
}

func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-Id"))
		if requestID == "" {
			requestID = strings.TrimSpace(c.GetHeader("X-Request-ID"))
		}
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Header("X-Request-Id", requestID)
		c.Header("X-Request-ID", requestID)
		ctx := logger.WithRequestID(c.Request.Context(), requestID)
		ctx = logger.InjectLogger(ctx, logger.Logger)
		if span := trace.SpanFromContext(ctx); span != nil {
			if sc := span.SpanContext(); sc.IsValid() {
				c.Header("X-Trace-Id", sc.TraceID().String())
			}
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func maxInt(value, fallback int) int {
	if value < fallback {
		return fallback
	}
	return value
}

func maxInt64(value, fallback int64) int64 {
	if value < fallback {
		return fallback
	}
	return value
}

func httpMetricAttributes(c *gin.Context, status int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("service", logger.ServiceName()),
		attribute.String("environment", logger.Environment()),
		attribute.String("method", c.Request.Method),
		attribute.String("route", routeLabel(c)),
		attribute.String("status_code", httpStatusCodeLabel(status)),
	}
}

func httpStatusCodeLabel(status int) string {
	if status <= 0 {
		return "in_flight"
	}
	return strconv.Itoa(status)
}

func httpResult(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	default:
		return "2xx"
	}
}
