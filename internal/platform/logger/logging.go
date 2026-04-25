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

type loggerKeyType struct{}
type requestIDKeyType struct{}

var loggerKey = loggerKeyType{}
var requestIDKey = requestIDKeyType{}

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

	Logger = withEnvFields(logger)
}

func InjectLogger(ctx context.Context, base *zap.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if base == nil {
		base = Logger
	}

	log := base
	span := trace.SpanFromContext(ctx)
	if sc := span.SpanContext(); sc.IsValid() {
		log = log.With(
			zap.String("trace_id", sc.TraceID().String()),
			zap.String("span_id", sc.SpanID().String()),
		)
	}
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		log = log.With(zap.String("request_id", requestID))
	}

	return context.WithValue(ctx, loggerKey, log)
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
		os.Getenv("SERVICE_NAME"),
		os.Getenv("OTEL_SERVICE_NAME"),
		"devflow",
	)
}

func Environment() string {
	return firstNonEmpty(
		os.Getenv("ENV"),
		os.Getenv("ENVIRONMENT"),
		os.Getenv("DEPLOYMENT_ENVIRONMENT"),
		"unknown",
	)
}

func ServiceVersion() string {
	return firstNonEmpty(
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

func withEnvFields(l *zap.Logger) *zap.Logger {
	fields := []zap.Field{
		zap.String("service", ServiceName()),
		zap.String("environment", Environment()),
		zap.String("service_version", ServiceVersion()),
		hostField(),
		envField("pod", "POD_NAME"),
		envField("namespace", "POD_NAMESPACE"),
		envField("node", "NODE_NAME"),
		envField("cluster", "CLUSTER_NAME"),
	}

	out := l
	for _, f := range fields {
		if f.Key != "" {
			out = out.With(f)
		}
	}
	return out
}

func hostField() zap.Field {
	if v := os.Getenv("HOSTNAME"); v != "" {
		return zap.String("host", v)
	}
	if v := os.Getenv("NODE_NAME"); v != "" {
		return zap.String("host", v)
	}
	return zap.Field{}
}

func envField(key, envKey string) zap.Field {
	if v := os.Getenv(envKey); v != "" {
		return zap.String(key, v)
	}
	return zap.Field{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
