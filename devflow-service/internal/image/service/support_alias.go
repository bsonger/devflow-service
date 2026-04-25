package service

import (
	"context"
	"database/sql"

	"github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	dbsql "github.com/bsonger/devflow-service/internal/platform/dbsql"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type RuntimeConfig = releasesupport.RuntimeConfig
type applicationProjection = releasesupport.ApplicationProjection

var ApplicationService = releasesupport.ApplicationService

func CurrentRuntimeConfig() releasesupport.RuntimeConfig {
	return releasesupport.CurrentRuntimeConfig()
}

func ConfigureRuntimeConfig(cfg releasesupport.RuntimeConfig) {
	releasesupport.ConfigureRuntimeConfig(cfg)
}

func StartServiceSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return observability.StartServiceSpan(ctx, spanName, opts...)
}

func marshalJSON(value any, empty string) ([]byte, error) {
	return dbsql.MarshalJSON(value, empty)
}

func nullableUUIDPtr(id *uuid.UUID) any {
	return dbsql.NullableUUIDPtr(id)
}

func placeholderClause(column string, position int) string {
	return dbsql.PlaceholderClause(column, position)
}

func ensureRowsAffected(result sql.Result) error {
	return dbsql.EnsureRowsAffected(result)
}

func scanImage(scanner interface{ Scan(dest ...any) error }) (*imagedomain.Image, error) {
	return ScanImage(scanner)
}
