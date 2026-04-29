package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

type BundleStore interface {
	Insert(ctx context.Context, bundle *model.ReleaseBundleRecord) error
	GetByReleaseID(ctx context.Context, releaseID uuid.UUID) (*model.ReleaseBundleRecord, error)
}

type bundlePostgresStore struct{}

func NewBundlePostgresStore() BundleStore {
	return &bundlePostgresStore{}
}

func (s *bundlePostgresStore) Insert(ctx context.Context, bundle *model.ReleaseBundleRecord) error {
	renderedObjectsJSON, err := dbsql.MarshalJSON(bundle.RenderedObjects, "[]")
	if err != nil {
		return err
	}
	_, err = db.DB().ExecContext(ctx, `
		insert into release_bundles (
			id, release_id, namespace, artifact_name, bundle_digest, rendered_objects, bundle_yaml, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, bundle.ID, bundle.ReleaseID, bundle.Namespace, bundle.ArtifactName, bundle.BundleDigest, renderedObjectsJSON, bundle.BundleYAML, bundle.CreatedAt, bundle.UpdatedAt, bundle.DeletedAt)
	return err
}

func (s *bundlePostgresStore) GetByReleaseID(ctx context.Context, releaseID uuid.UUID) (*model.ReleaseBundleRecord, error) {
	return scanReleaseBundleRecord(db.DB().QueryRowContext(ctx, `
		select id, release_id, namespace, artifact_name, bundle_digest, rendered_objects, bundle_yaml, created_at, updated_at, deleted_at
		from release_bundles
		where release_id = $1 and deleted_at is null
	`, releaseID))
}

func scanReleaseBundleRecord(scanner interface{ Scan(dest ...any) error }) (*model.ReleaseBundleRecord, error) {
	var (
		item                model.ReleaseBundleRecord
		renderedObjectsJSON []byte
		deletedAt           sql.NullTime
	)

	if err := scanner.Scan(
		&item.ID,
		&item.ReleaseID,
		&item.Namespace,
		&item.ArtifactName,
		&item.BundleDigest,
		&renderedObjectsJSON,
		&item.BundleYAML,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	if len(renderedObjectsJSON) > 0 {
		if err := json.Unmarshal(renderedObjectsJSON, &item.RenderedObjects); err != nil {
			return nil, err
		}
	}
	item.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	return &item, nil
}
