package logger

import (
	"context"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger
var RootLogger *zap.Logger

type loggerKeyType struct{}
type requestIDKeyType struct{}

var loggerKey = loggerKeyType{}
var requestIDKey = requestIDKeyType{}
var baseLoggerKey = struct{}{}

type Config struct {
	Level  string
	Format string
}

func InitZapLogger(config *Config) {
	if config == nil {
		panic("InitZapLogger: log config is nil")
	}

	format := strings.ToLower(strings.TrimSpace(config.Format))
	var cfg zap.Config
	if format == "" || format == "json" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}

	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.LevelKey = "severity_text"
	cfg.EncoderConfig.MessageKey = "body"
	cfg.EncoderConfig.NameKey = "logger.name"
	cfg.EncoderConfig.StacktraceKey = "stacktrace"

	level := zapcore.InfoLevel
	if config.Level != "" {
		_ = level.Set(strings.ToLower(config.Level))
	}
	cfg.Level = zap.NewAtomicLevelAt(level)

	cfg.DisableStacktrace = false
	cfg.DisableCaller = false
	cfg.Development = format != "" && format != "json"

	logger, err := cfg.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		panic(err)
	}

	RootLogger = withResourceFields(logger)
	Logger = RootLogger.Named(ServiceName())
}

func InjectLogger(ctx context.Context, base *zap.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	baseRoot := RootLogger
	if baseRoot == nil {
		baseRoot = zap.NewNop()
	}
	if base == nil {
		base = Logger
	}
	if base == nil {
		base = zap.NewNop()
	}

	log := base
	rootLog := baseRoot
	span := trace.SpanFromContext(ctx)
	if sc := span.SpanContext(); sc.IsValid() {
		log = log.With(
			zap.String("trace_id", sc.TraceID().String()),
			zap.String("span_id", sc.SpanID().String()),
		)
		rootLog = rootLog.With(
			zap.String("trace_id", sc.TraceID().String()),
			zap.String("span_id", sc.SpanID().String()),
		)
	}
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		log = log.With(zap.String("request_id", requestID))
	}

	ctx = context.WithValue(ctx, loggerKey, log)
	return context.WithValue(ctx, baseLoggerKey, rootLog)
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

func ServiceName() string {
	return firstNonEmpty(
		os.Getenv("OTEL_SERVICE_NAME"),
		resourceAttribute("service.name"),
		os.Getenv("SERVICE_NAME"),
		"devflow",
	)
}

func ServiceNamespace() string {
	return firstNonEmpty(
		resourceAttribute("service.namespace"),
		os.Getenv("OTEL_SERVICE_NAMESPACE"),
		"devflow",
	)
}

func Environment() string {
	return DeploymentEnvironmentName()
}

func DeploymentEnvironmentName() string {
	return firstNonEmpty(
		resourceAttribute("deployment.environment.name"),
		resourceAttribute("deployment.environment"),
		os.Getenv("DEPLOYMENT_ENVIRONMENT"),
		os.Getenv("ENVIRONMENT"),
		os.Getenv("ENV"),
		"unknown",
	)
}

func ServiceVersion() string {
	return normalizeServiceVersion(resolvedServiceVersion())
}

func ContainerImageDigest() string {
	resolved := resolvedServiceVersion()
	if !isDigestValue(resolved) {
		return ""
	}
	return resolved
}

func resolvedServiceVersion() string {
	return firstNonEmpty(
		resourceAttribute("service.version"),
		os.Getenv("SERVICE_VERSION"),
		os.Getenv("VERSION"),
		"unknown",
	)
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}

func LoggerFromContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return Logger
	}
	if l, ok := ctx.Value(loggerKey).(*zap.Logger); ok {
		return l
	}
	return Logger
}

func LoggerWithContext(ctx context.Context) *zap.Logger {
	return LoggerFromContext(ctx)
}

func BaseLoggerFromContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return RootLogger
	}
	if l, ok := ctx.Value(baseLoggerKey).(*zap.Logger); ok {
		return l
	}
	return RootLogger
}

func NamedLoggerFromContext(ctx context.Context, name string) *zap.Logger {
	base := BaseLoggerFromContext(ctx)
	if base == nil {
		base = zap.NewNop()
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return base
	}
	return base.Named(name)
}

type ZapAdapter struct {
	logger *zap.Logger
	sugar  *zap.SugaredLogger
}

func NewZapAdapter(logger *zap.Logger) *ZapAdapter {
	if logger == nil {
		logger = Logger
	}
	return &ZapAdapter{
		logger: logger,
		sugar:  logger.Sugar(),
	}
}

func (z *ZapAdapter) Infof(msg string, args ...interface{}) {
	z.sugar.Infof(msg, args...)
}

func (z *ZapAdapter) Debugf(msg string, args ...interface{}) {
	z.sugar.Debugf(msg, args...)
}

func (z *ZapAdapter) Errorf(msg string, args ...interface{}) {
	z.sugar.Errorf(msg, args...)
}

func withResourceFields(l *zap.Logger) *zap.Logger {
	fields := []zap.Field{
		zap.String("service.name", ServiceName()),
		zap.String("service.namespace", ServiceNamespace()),
		zap.String("service.version", ServiceVersion()),
	}

	out := l
	for _, f := range fields {
		if f.Key != "" {
			out = out.With(f)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func resourceAttribute(key string) string {
	attrs := parseResourceAttributes(os.Getenv("OTEL_RESOURCE_ATTRIBUTES"))
	return attrs[key]
}

func parseResourceAttributes(value string) map[string]string {
	attrs := map[string]string{}
	for _, part := range strings.Split(value, ",") {
		key, raw, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		raw = strings.TrimSpace(raw)
		if key == "" || raw == "" {
			continue
		}
		attrs[key] = raw
	}
	return attrs
}

func normalizeServiceVersion(value string) string {
	value = strings.TrimSpace(value)
	if !isDigestValue(value) {
		return value
	}
	algorithm, digest, ok := strings.Cut(value, ":")
	if !ok || digest == "" {
		return value
	}
	if len(digest) > 12 {
		digest = digest[:12]
	}
	return algorithm + ":" + digest
}

func isDigestValue(value string) bool {
	algorithm, digest, ok := strings.Cut(strings.TrimSpace(value), ":")
	if !ok || algorithm == "" || digest == "" {
		return false
	}
	if len(digest) < 16 {
		return false
	}
	for _, r := range digest {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
