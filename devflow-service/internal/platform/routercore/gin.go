package routercore

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	platformhttpx "github.com/bsonger/devflow-service/internal/platform/httpx"
	loggingx "github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/grafana/pyroscope-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

func ShouldIgnorePath(path string) bool {
	return path == "/metrics" ||
		path == "/health" ||
		path == "/healthz" ||
		path == "/readyz" ||
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
	httpMetricsOnce     sync.Once
	httpRequestsCounter metric.Int64Counter
	httpErrorsCounter   metric.Int64Counter
	httpRequestLatency  metric.Float64Histogram
	httpRequestSize     metric.Int64Histogram
	httpResponseSize    metric.Int64Histogram
	httpMetricsInitErr  error
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

		c.Next()

		if httpMetricsInitErr != nil {
			return
		}

		status := c.Writer.Status()
		statusClass := httpStatusClass(status)
		attrs := []attribute.KeyValue{
			attribute.String("service", serviceLabel()),
			attribute.String("route", routeLabel(c)),
			attribute.String("method", c.Request.Method),
			attribute.String("status_class", statusClass),
		}

		ctx := c.Request.Context()
		httpRequestsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		httpRequestLatency.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		httpRequestSize.Record(ctx, requestSize, metric.WithAttributes(attrs...))
		httpResponseSize.Record(ctx, int64(maxInt(c.Writer.Size(), 0)), metric.WithAttributes(attrs...))
		if status >= 400 {
			httpErrorsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
	}
}

func initHTTPMetrics() {
	meter := otel.Meter("devflow/http")

	httpRequestsCounter, httpMetricsInitErr = meter.Int64Counter(
		"devflow_http_requests_total",
		metric.WithUnit("{request}"),
	)
	if httpMetricsInitErr != nil {
		return
	}
	httpErrorsCounter, httpMetricsInitErr = meter.Int64Counter(
		"devflow_http_errors_total",
		metric.WithUnit("{error}"),
	)
	if httpMetricsInitErr != nil {
		return
	}

	httpRequestLatency, httpMetricsInitErr = meter.Float64Histogram(
		"devflow_http_request_duration_seconds",
		metric.WithUnit("s"),
	)
	if httpMetricsInitErr != nil {
		return
	}

	httpRequestSize, httpMetricsInitErr = meter.Int64Histogram(
		"http.server.request.size",
		metric.WithUnit("By"),
	)
	if httpMetricsInitErr != nil {
		return
	}

	httpResponseSize, httpMetricsInitErr = meter.Int64Histogram(
		"http.server.response.size",
		metric.WithUnit("By"),
	)
}

func GinZapLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		req := c.Request
		path := req.URL.Path
		rawQuery := req.URL.RawQuery

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
			zap.String("http.method", req.Method),
			zap.String("http.route", route),
			zap.String("http.target", buildTarget(path, rawQuery)),
			zap.Int("http.status_code", status),
			zap.String("client.ip", c.ClientIP()),
			zap.String("user_agent.original", req.UserAgent()),
			zap.Duration("http.server.duration", latency),
		}

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			fields = append(fields, zap.String("error.message", err.Error()))
		}

		logger := loggingx.LoggerFromContext(req.Context())

		switch {
		case status >= 500:
			logger.Error("http request", fields...)
		case status >= 400:
			logger.Warn("http request", fields...)
		case latency >= time.Second:
			logger.Warn("slow http request", fields...)
		default:
			logger.Info("http request", fields...)
		}
	}
}

func GinZapRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				logger := loggingx.LoggerFromContext(c.Request.Context())
				logger.Error("panic recovered",
					zap.String("component", "http_server"),
					zap.String("result", "panic"),
					zap.Any("panic", rec),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.String("client_ip", c.ClientIP()),
				)
				platformhttpx.WriteError(c, http.StatusInternalServerError, "internal", "internal server error", nil)
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
			requestID = uuid.NewString()
		}
		c.Header("X-Request-Id", requestID)
		ctx := loggingx.WithRequestID(c.Request.Context(), requestID)
		ctx = loggingx.InjectLogger(ctx, loggingx.Logger)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func buildTarget(path, rawQuery string) string {
	if rawQuery == "" {
		return path
	}
	return path + "?" + rawQuery
}

func maxInt(value, fallback int) int {
	if value < fallback {
		return fallback
	}
	return value
}

func serviceLabel() string {
	if name := os.Getenv("SERVICE_NAME"); name != "" {
		return name
	}
	return "devflow"
}

func httpStatusClass(status int) string {
	return httpResult(status)
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
