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
	if gotBase := BaseLoggerFromContext(ctx); gotBase == nil {
		t.Fatal("expected base logger from context")
	}
}

func TestNewZapAdapterFallsBackToGlobalLogger(t *testing.T) {
	Logger = zap.NewNop()
	adapter := NewZapAdapter(nil)
	if adapter == nil || adapter.logger == nil {
		t.Fatal("expected adapter with logger")
	}
}

func TestResourceFieldsPreferOTELResourceAttributes(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("OTEL_SERVICE_NAMESPACE", "")
	t.Setenv("SERVICE_VERSION", "")
	t.Setenv("DEPLOYMENT_ENVIRONMENT", "")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.name=config-service,service.namespace=devflow,service.version=v1,deployment.environment.name=pre-production")

	if got := ServiceName(); got != "config-service" {
		t.Fatalf("ServiceName() = %q, want config-service", got)
	}
	if got := ServiceNamespace(); got != "devflow" {
		t.Fatalf("ServiceNamespace() = %q, want devflow", got)
	}
	if got := ServiceVersion(); got != "v1" {
		t.Fatalf("ServiceVersion() = %q, want v1", got)
	}
	if got := DeploymentEnvironmentName(); got != "pre-production" {
		t.Fatalf("DeploymentEnvironmentName() = %q, want pre-production", got)
	}
}

func TestDeploymentEnvironmentAcceptsLegacyResourceAttributeFallback(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "deployment.environment=pre-production")
	t.Setenv("DEPLOYMENT_ENVIRONMENT", "")
	t.Setenv("ENVIRONMENT", "")
	t.Setenv("ENV", "")

	if got := DeploymentEnvironmentName(); got != "pre-production" {
		t.Fatalf("DeploymentEnvironmentName() = %q, want pre-production", got)
	}
}

func TestServiceVersionNormalizesDigestAndExposesFullContainerDigest(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")
	t.Setenv("SERVICE_VERSION", "sha256:8fc33fd48da9be177f5d75bf55ed6a6a39cd0a99d841a7604f5e897e50031f52")
	t.Setenv("VERSION", "")

	if got := ServiceVersion(); got != "sha256:8fc33fd48da9" {
		t.Fatalf("ServiceVersion() = %q", got)
	}
}
