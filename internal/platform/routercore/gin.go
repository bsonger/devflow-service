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
	platformotel "github.com/bsonger/devflow-service/internal/platform/otel"
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
		path == "/livez" ||
		path == "/readyz" ||
		path == "/favicon.ico" ||
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
		start := time.Now()
		path := c.Request.URL.Path
		requestSize := c.Request.ContentLength
		if requestSize < 0 {
			requestSize = 0
		}
		ctx := c.Request.Context()
		startAttrs := httpMetricAttributes(c, 0)
		trackInFlight := !ShouldIgnorePath(path)
		if httpMetricsInitErr == nil && trackInFlight {
			httpRequestsInFlight.Add(ctx, 1, metric.WithAttributes(startAttrs...))
			defer httpRequestsInFlight.Add(ctx, -1, metric.WithAttributes(startAttrs...))
		}

		c.Next()

		if httpMetricsInitErr != nil {
			return
		}

		status := c.Writer.Status()
		latency := time.Since(start)
		if shouldSkipHTTPMetric(path, status, latency) {
			return
		}
		if status >= 500 {
			ctx = platformotel.WithMetricExemplar(ctx)
		}
		attrs := httpMetricAttributes(c, status)
		httpRequestsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		httpRequestLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(attrs...))
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
		metric.WithDescription("Duration of HTTP server requests."),
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
		if shouldSkipHTTPRequestLog(path, status, latency) {
			return
		}
		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}

		fields := []zap.Field{
			zap.String("http.request.method", req.Method),
			zap.String("http.route", route),
			zap.String("url.path", path),
			zap.Int("http.response.status_code", status),
			zap.Int("http.response.body.size", maxInt(c.Writer.Size(), 0)),
			zap.Float64("duration_ms", float64(latency)/float64(time.Millisecond)),
			zap.Float64("http.server.request.duration", latency.Seconds()),
			zap.String("client.address", c.ClientIP()),
			zap.String("user_agent.original", req.UserAgent()),
		}
		if bodySize, ok := requestBodySizeField(req.Method, req.ContentLength); ok {
			fields = append(fields, bodySize)
		}
		if shouldIncludeDevflowAccessFields(status, latency) {
			fields = append(fields, devflowIdentityFields(c, route)...)
		}

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			fields = append(fields, zap.String("error_message", err.Error()))
		}

		log := httpRequestLogger(req.Context(), status)
		message := httpRequestMessage(status, latency)

		switch {
		case status >= 500:
			log.Error(message, fields...)
		case status >= 400:
			log.Warn(message, fields...)
		case latency >= time.Second:
			log.Warn(message, fields...)
		default:
			log.Info(message, fields...)
		}
	}
}

func GinZapRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log := logger.NamedLoggerFromContext(c.Request.Context(), "http.error")
				log.Error("panic recovered",
					zap.Any("panic", rec),
					zap.String("http.request.method", c.Request.Method),
					zap.String("http.route", routeLabel(c)),
					zap.String("url.path", c.Request.URL.Path),
					zap.String("client.address", c.ClientIP()),
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
		attribute.String("service_name", logger.ServiceName()),
		attribute.String("service_namespace", logger.ServiceNamespace()),
		attribute.String("deployment_environment_name", logger.Environment()),
		attribute.String("http_request_method", c.Request.Method),
		attribute.String("http_route", routeLabel(c)),
		attribute.String("http_response_status_code", httpStatusCodeLabel(status)),
		attribute.String("http_response_status_class", httpStatusClass(status)),
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

func httpStatusClass(status int) string {
	if status <= 0 {
		return "in_flight"
	}
	return strconv.Itoa(status/100) + "xx"
}

func requestBodySizeField(method string, size int64) (zap.Field, bool) {
	size = maxInt64(size, 0)
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead:
		if size == 0 {
			return zap.Skip(), false
		}
	}
	return zap.Int64("http.request.body.size", size), true
}

func shouldIncludeDevflowAccessFields(status int, latency time.Duration) bool {
	if status >= 400 {
		return true
	}
	return latency >= time.Second
}

func httpRequestLogger(ctx context.Context, status int) *zap.Logger {
	switch {
	case status >= 400:
		return logger.NamedLoggerFromContext(ctx, "http.error")
	default:
		return logger.NamedLoggerFromContext(ctx, "http.access")
	}
}

func httpRequestMessage(status int, latency time.Duration) string {
	switch {
	case status >= 500:
		return "http server error"
	case status >= 400:
		return "http client error"
	case latency >= time.Second:
		return "slow http request"
	default:
		return "http request"
	}
}

func shouldSkipHTTPMetric(path string, status int, latency time.Duration) bool {
	return shouldSkipHTTPRequestLog(path, status, latency)
}

func shouldSkipHTTPRequestLog(path string, status int, latency time.Duration) bool {
	if !ShouldIgnorePath(path) {
		return false
	}
	if status >= 400 {
		return false
	}
	return latency < time.Second
}

func devflowIdentityFields(c *gin.Context, route string) []zap.Field {
	values := map[string]string{
		"devflow.project.id":     firstRequestValue(c, "X-Devflow-Project-Id", "project_id", "project_id"),
		"devflow.application.id": firstRequestValue(c, "X-Devflow-Application-Id", "application_id", "application_id"),
		"devflow.service.id":     firstRequestValue(c, "X-Devflow-Service-Id", "service_id", "service_id"),
		"devflow.environment.id": firstRequestValue(c, "X-Devflow-Environment-Id", "environment_id", "environment_id"),
		"devflow.release.id":     firstRequestValue(c, "X-Devflow-Release-Id", "release_id", "release_id"),
		"devflow.manifest.id":    firstRequestValue(c, "X-Devflow-Manifest-Id", "manifest_id", "manifest_id"),
	}

	if id := strings.TrimSpace(c.Param("id")); id != "" {
		switch {
		case strings.Contains(route, "/projects/:id"):
			values["devflow.project.id"] = id
		case strings.Contains(route, "/applications/:id"):
			values["devflow.application.id"] = id
		case strings.Contains(route, "/services/:id"):
			values["devflow.service.id"] = id
		case strings.Contains(route, "/environments/:id"):
			values["devflow.environment.id"] = id
		case strings.Contains(route, "/releases/:id"):
			values["devflow.release.id"] = id
		case strings.Contains(route, "/manifests/:id"):
			values["devflow.manifest.id"] = id
		}
	}

	fields := make([]zap.Field, 0, len(values))
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		fields = append(fields, zap.String(key, strings.TrimSpace(value)))
	}
	return fields
}

func firstRequestValue(c *gin.Context, headerName, queryName, paramName string) string {
	for _, value := range []string{
		c.GetHeader(headerName),
		c.Query(queryName),
		c.Param(paramName),
	} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
