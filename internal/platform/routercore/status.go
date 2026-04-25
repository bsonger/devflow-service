package routercore

import (
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	platformobservability "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

var processStartedAt = time.Now().UTC()

type StatusOptions struct {
	ServiceName   string
	EnableSwagger bool
	Modules       []string
}

func RegisterStatusRoutes(r *gin.Engine, opts StatusOptions) {
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, statusPayload(c, opts, "ok"))
	})

	r.GET("/readyz", func(c *gin.Context) {
		c.JSON(200, statusPayload(c, opts, "ready"))
	})

	r.GET("/internal/status", func(c *gin.Context) {
		c.JSON(200, statusPayload(c, opts, "ok"))
	})
}

func statusPayload(c *gin.Context, opts StatusOptions, status string) gin.H {
	ctx := c.Request.Context()
	payload := gin.H{
		"service":     opts.ServiceName,
		"environment": logger.Environment(),
		"version":     logger.ServiceVersion(),
		"status":      status,
		"started_at":  processStartedAt.Format(time.RFC3339),
		"uptime_sec":  int64(time.Since(processStartedAt).Seconds()),
		"request_id":  logger.RequestIDFromContext(ctx),
		"runtime": gin.H{
			"pid":        os.Getpid(),
			"go_version": runtime.Version(),
		},
		"http": gin.H{
			"swagger_enabled": opts.EnableSwagger,
			"modules":         normalizedModules(opts.Modules),
		},
		"otel": gin.H{
			"service_name":   logger.ServiceName(),
			"endpoint_set":   strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != "",
			"protocol":       strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")),
			"resource_attrs": strings.TrimSpace(os.Getenv("OTEL_RESOURCE_ATTRIBUTES")) != "",
			"sampler":        strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER")),
			"sampler_arg":    strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG")),
		},
	}

	if span := trace.SpanFromContext(ctx); span != nil {
		if sc := span.SpanContext(); sc.IsValid() {
			payload["trace_id"] = sc.TraceID().String()
			payload["span_id"] = sc.SpanID().String()
		}
	}

	if lastFailure := platformobservability.LastFailure(); lastFailure != nil {
		payload["last_failure"] = lastFailure
	}

	return payload
}

func normalizedModules(modules []string) []string {
	if len(modules) == 0 {
		return []string{}
	}

	out := make([]string, 0, len(modules))
	seen := make(map[string]struct{}, len(modules))
	for _, module := range modules {
		module = strings.TrimSpace(module)
		if module == "" {
			continue
		}
		if _, ok := seen[module]; ok {
			continue
		}
		seen[module] = struct{}{}
		out = append(out, module)
	}
	slices.Sort(out)
	return out
}
