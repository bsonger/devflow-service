package service

import (
	"context"
	"database/sql"

	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/google/uuid"
)

var ApplicationService = releasesupport.ApplicationService

var (
	ErrDeployTargetBindingMissing               = releasesupport.ErrDeployTargetBindingMissing
	ErrDeployTargetBindingMalformed             = releasesupport.ErrDeployTargetBindingMalformed
	ErrDeployTargetApplicationMetadataMissing   = releasesupport.ErrDeployTargetApplicationMetadataMissing
	ErrDeployTargetApplicationMetadataMalformed = releasesupport.ErrDeployTargetApplicationMetadataMalformed
	ErrDeployTargetProjectMetadataMissing       = releasesupport.ErrDeployTargetProjectMetadataMissing
	ErrDeployTargetProjectMetadataMalformed     = releasesupport.ErrDeployTargetProjectMetadataMalformed
	ErrDeployTargetEnvironmentMetadataMissing   = releasesupport.ErrDeployTargetEnvironmentMetadataMissing
	ErrDeployTargetEnvironmentMetadataMalformed = releasesupport.ErrDeployTargetEnvironmentMetadataMalformed
	ErrDeployTargetClusterMetadataMissing       = releasesupport.ErrDeployTargetClusterMetadataMissing
	ErrDeployTargetClusterMetadataMalformed     = releasesupport.ErrDeployTargetClusterMetadataMalformed
	ErrDeployTargetClusterNotReady              = releasesupport.ErrDeployTargetClusterNotReady
	ErrDeployTargetClusterReadinessMalformed    = releasesupport.ErrDeployTargetClusterReadinessMalformed
	ErrDeployTargetNamespaceInvalid             = releasesupport.ErrDeployTargetNamespaceInvalid
	ErrDeployTargetClusterServerInvalid         = releasesupport.ErrDeployTargetClusterServerInvalid
)

type applicationProjection = releasesupport.ApplicationProjection
type deployTarget = releasesupport.DeployTarget

func resolveDeployTarget(ctx context.Context, applicationID, environmentID string) (*releasesupport.DeployTarget, error) {
	return releasesupport.ResolveDeployTarget(ctx, applicationID, environmentID)
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

func scanRelease(scanner interface{ Scan(dest ...any) error }) (*model.Release, error) {
	return ScanRelease(scanner)
}
