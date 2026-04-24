package logger

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestInjectLoggerAddsRequestIDFromContext(t *testing.T) {
	InitZapLogger(&Config{Level: "info", Format: "json"})

	ctx := WithRequestID(context.Background(), "req-123")
	ctx = InjectLogger(ctx, Logger)
	logger := LoggerFromContext(ctx)

	if logger == nil {
		t.Fatal("expected logger from context")
	}
}

func TestRequestIDFromContext(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-456")
	if got := RequestIDFromContext(ctx); got != "req-456" {
		t.Fatalf("request id = %q, want %q", got, "req-456")
	}
}

func TestNewZapAdapterFallsBackToGlobalLogger(t *testing.T) {
	Logger = zap.NewNop()
	adapter := NewZapAdapter(nil)
	if adapter == nil || adapter.logger == nil {
		t.Fatal("expected adapter with logger")
	}
}
