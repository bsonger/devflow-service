package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/k8s"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	dbsql "github.com/bsonger/devflow-service/internal/platform/dbsql"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
)

type applicationProjection = releasesupport.ApplicationProjection

var ApplicationService = releasesupport.ApplicationService

func marshalJSON(value any, empty string) ([]byte, error) {
	return dbsql.MarshalJSON(value, empty)
}

func CurrentRuntimeConfig() releasesupport.RuntimeConfig {
	return releasesupport.CurrentRuntimeConfig()
}

func resolveDeployTarget(ctx context.Context, applicationID, environmentID string) (*releasesupport.DeployTarget, error) {
	return releasesupport.ResolveDeployTarget(ctx, applicationID, environmentID)
}

func deriveNamespace(projectName, environmentName string) (string, error) {
	return k8s.DeriveNamespace(projectName, environmentName)
}

func placeholderClause(column string, position int) string {
	return dbsql.PlaceholderClause(column, position)
}

func ensureRowsAffected(result sql.Result) error {
	return dbsql.EnsureRowsAffected(result)
}

func nullableTimePtr(t *time.Time) any {
	return dbsql.NullableTimePtr(t)
}

func scanManifest(scanner interface{ Scan(dest ...any) error }) (*manifestdomain.Manifest, error) {
	return ScanManifest(scanner)
}
