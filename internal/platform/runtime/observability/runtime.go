package observability

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/internal/platform/otel"
	"github.com/bsonger/devflow-service/internal/platform/runtime/pyroscopex"
	gootel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type RuntimeOptions struct {
	LogLevel               string
	LogFormat              string
	OtelEndpoint           string
	OtelProtocol           string
	OtelService            string
	OtelResourceAttributes string
	OtelSampleRatio        float64
	PyroscopeAddr          string
	ServiceOverride        string
}

var (
	runtimeMetricsOnce               sync.Once
	runtimeMetricsInitErr            error
	runtimeReleaseReconcileTotal     metric.Int64Counter
	runtimeReleaseWritebackTotal     metric.Int64Counter
	runtimeTerminalLabelUpdateTotal  metric.Int64Counter
	runtimeObservedWorkloadStateTotal metric.Int64Counter
)

func Init(ctx context.Context, opts RuntimeOptions) (func(context.Context) error, error) {
	serviceName := ResolveServiceName(opts.ServiceOverride, opts.OtelService)
	if serviceName != "" {
		_ = os.Setenv("SERVICE_NAME", serviceName)
	}
	if strings.TrimSpace(opts.OtelEndpoint) != "" && strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) == "" {
		_ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", opts.OtelEndpoint)
	}
	if strings.TrimSpace(opts.OtelProtocol) != "" && strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")) == "" {
		_ = os.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", opts.OtelProtocol)
	}

	logger.InitZapLogger(&logger.Config{
		Level:  opts.LogLevel,
		Format: opts.LogFormat,
	})
	gootel.SetErrorHandler(gootel.ErrorHandlerFunc(func(err error) {
		if err == nil {
			return
		}
		logger.RootLogger.Named("otel.exporter").Error("OpenTelemetry runtime error", zap.Error(err))
	}))

	shutdown := func(context.Context) error { return nil }
	if opts.OtelEndpoint != "" {
		tpShutdown, err := otel.InitOtel(ctx, &otel.Config{
			Endpoint:           opts.OtelEndpoint,
			Protocol:           opts.OtelProtocol,
			ServiceName:        serviceName,
			ResourceAttributes: opts.OtelResourceAttributes,
			SampleRatio:        opts.OtelSampleRatio,
		})
		if err != nil {
			return nil, err
		}
		shutdown = tpShutdown
	}

	if opts.PyroscopeAddr != "" {
		pyroscopex.InitPyroscope(serviceName, opts.PyroscopeAddr)
	}

	if err := otel.InitMetricProvider(); err != nil {
		return shutdown, err
	}

	return shutdown, nil
}

func ResolveServiceName(override, configServiceName string) string {
	if override != "" {
		return override
	}
	if configServiceName != "" {
		return configServiceName
	}
	return "devflow"
}

func ReinjectLogger(ctx context.Context) context.Context {
	return logger.InjectLogger(ctx, logger.LoggerFromContext(ctx))
}

func StartSpan(ctx context.Context, tracer trace.Tracer, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, spanName, opts...)
	return ReinjectLogger(ctx), span
}

var devflowTracer = gootel.Tracer("devflow")

func StartServiceSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return StartSpan(ctx, devflowTracer, spanName, opts...)
}

func StartWorkerSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return StartSpan(ctx, gootel.Tracer("release-worker"), spanName, opts...)
}

func RecordRuntimeReleaseReconcile(ctx context.Context, phase, result string) {
	runtimeMetricsOnce.Do(initRuntimeReleaseMetrics)
	if runtimeMetricsInitErr != nil {
		return
	}
	phase = normalizeRuntimePhaseLabel(phase)
	result = normalizeRuntimeResultLabel(result)
	if phase == "" || result == "" {
		return
	}
	runtimeReleaseReconcileTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("phase", phase),
		attribute.String("result", result),
	))
}

func RecordRuntimeReleaseWriteback(ctx context.Context, stepCode, result, statusCode string) {
	runtimeMetricsOnce.Do(initRuntimeReleaseMetrics)
	if runtimeMetricsInitErr != nil {
		return
	}
	stepCode = normalizeRuntimeStepCodeLabel(stepCode)
	result = normalizeRuntimeResultLabel(result)
	statusCode = normalizeRuntimeStatusCodeLabel(statusCode)
	if stepCode == "" || result == "" || statusCode == "" {
		return
	}
	runtimeReleaseWritebackTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("step_code", stepCode),
		attribute.String("result", result),
		attribute.String("http_response_status_code", statusCode),
	))
}

func RecordRuntimeTerminalLabelUpdate(ctx context.Context, workloadKind, result, errorCode string) {
	runtimeMetricsOnce.Do(initRuntimeReleaseMetrics)
	if runtimeMetricsInitErr != nil {
		return
	}
	workloadKind = normalizeRuntimeWorkloadKindLabel(workloadKind)
	result = normalizeRuntimeResultLabel(result)
	errorCode = normalizeRuntimeErrorCodeLabel(errorCode)
	if workloadKind == "" || result == "" || errorCode == "" {
		return
	}
	runtimeTerminalLabelUpdateTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("workload_kind", workloadKind),
		attribute.String("result", result),
		attribute.String("error_code", errorCode),
	))
}

func RecordRuntimeObservedWorkloadState(ctx context.Context, summaryStatus, observeState string) {
	runtimeMetricsOnce.Do(initRuntimeReleaseMetrics)
	if runtimeMetricsInitErr != nil {
		return
	}
	summaryStatus = normalizeRuntimeSummaryStatusLabel(summaryStatus)
	observeState = normalizeRuntimeObserveStateLabel(observeState)
	if summaryStatus == "" || observeState == "" {
		return
	}
	runtimeObservedWorkloadStateTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("summary_status", summaryStatus),
		attribute.String("observe_state", observeState),
	))
}

func initRuntimeReleaseMetrics() {
	meter := gootel.Meter("devflow/runtime")
	runtimeReleaseReconcileTotal, runtimeMetricsInitErr = meter.Int64Counter("runtime_release_reconcile_total", metric.WithUnit("{reconcile}"))
	if runtimeMetricsInitErr != nil {
		return
	}
	runtimeReleaseWritebackTotal, runtimeMetricsInitErr = meter.Int64Counter("runtime_release_writeback_total", metric.WithUnit("{writeback}"))
	if runtimeMetricsInitErr != nil {
		return
	}
	runtimeTerminalLabelUpdateTotal, runtimeMetricsInitErr = meter.Int64Counter("runtime_terminal_label_update_total", metric.WithUnit("{update}"))
	if runtimeMetricsInitErr != nil {
		return
	}
	runtimeObservedWorkloadStateTotal, runtimeMetricsInitErr = meter.Int64Counter("runtime_observed_workload_state_total", metric.WithUnit("{state}"))
}

func normalizeRuntimePhaseLabel(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "pending", "running", "succeeded", "failed":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "unknown"
	}
}

func normalizeRuntimeResultLabel(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "ok", "error", "requeue":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "unknown"
	}
}

func normalizeRuntimeStepCodeLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	value = strings.ReplaceAll(value, " ", "_")
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func normalizeRuntimeStatusCodeLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "none"
	}
	if len(value) > 16 {
		return value[:16]
	}
	return value
}

func normalizeRuntimeWorkloadKindLabel(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "deployment", "rollout":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "unknown"
	}
}

func normalizeRuntimeErrorCodeLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "none"
	}
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func normalizeRuntimeSummaryStatusLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func normalizeRuntimeObserveStateLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}
