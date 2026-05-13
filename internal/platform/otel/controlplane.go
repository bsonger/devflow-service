package otel

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/attribute"
)

func WithFailureMetricExemplarContext(ctx context.Context, failed bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if !failed {
		return ctx
	}
	return WithMetricExemplar(ctx)
}

func AppendBoundedLabel(attrs []attribute.KeyValue, key, value, fallback string) []attribute.KeyValue {
	key = strings.TrimSpace(key)
	if key == "" {
		return attrs
	}
	value = strings.TrimSpace(value)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	if value == "" {
		return attrs
	}
	return append(attrs, attribute.String(key, value))
}
