package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	"github.com/google/uuid"
)

type Store interface {
	Insert(ctx context.Context, manifest *manifestdomain.Manifest) error
	List(ctx context.Context, filter manifestdomain.ManifestListFilter) ([]manifestdomain.Manifest, error)
	Get(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type PostgresStore struct{}

func NewPostgresStore() Store {
	return &PostgresStore{}
}

func (s *PostgresStore) Insert(ctx context.Context, m *manifestdomain.Manifest) error {
	servicesJSON, err := dbsql.MarshalJSON(m.ServicesSnapshot, "[]")
	if err != nil {
		return err
	}
	workloadJSON, err := dbsql.MarshalJSON(m.WorkloadConfigSnapshot, "{}")
	if err != nil {
		return err
	}
	renderedJSON, err := dbsql.MarshalJSON(m.RenderedObjects, "[]")
	if err != nil {
		return err
	}
	_, err = db.DB().ExecContext(ctx, `
		insert into manifests (
			id, application_id, environment_id, image_id, image_ref,
			artifact_repository, artifact_tag, artifact_ref, artifact_digest, artifact_media_type, artifact_pushed_at,
			services_snapshot, workload_config_snapshot,
			rendered_objects, rendered_yaml, status, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
	`, m.ID, m.ApplicationID, m.EnvironmentID, m.ImageID, m.ImageRef,
		m.ArtifactRepository, m.ArtifactTag, m.ArtifactRef, m.ArtifactDigest, m.ArtifactMediaType, dbsql.NullableTimePtr(m.ArtifactPushedAt),
		servicesJSON, workloadJSON, renderedJSON, m.RenderedYAML,
		m.Status, m.CreatedAt, m.UpdatedAt, m.DeletedAt)
	return err
}

func (s *PostgresStore) List(ctx context.Context, filter manifestdomain.ManifestListFilter) ([]manifestdomain.Manifest, error) {
	query := `
		select id, application_id, environment_id, image_id, image_ref,
			artifact_repository, artifact_tag, artifact_ref, artifact_digest, artifact_media_type, artifact_pushed_at,
			services_snapshot, workload_config_snapshot,
			rendered_objects, rendered_yaml, status, created_at, updated_at, deleted_at
		from manifests
	`
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.ApplicationID != nil {
		args = append(args, *filter.ApplicationID)
		clauses = append(clauses, dbsql.PlaceholderClause("application_id", len(args)))
	}
	if filter.ImageID != nil {
		args = append(args, *filter.ImageID)
		clauses = append(clauses, dbsql.PlaceholderClause("image_id", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]manifestdomain.Manifest, 0)
	for rows.Next() {
		item, err := scanManifest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (s *PostgresStore) Get(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
	return scanManifest(db.DB().QueryRowContext(ctx, `
		select id, application_id, environment_id, image_id, image_ref,
			artifact_repository, artifact_tag, artifact_ref, artifact_digest, artifact_media_type, artifact_pushed_at,
			services_snapshot, workload_config_snapshot,
			rendered_objects, rendered_yaml, status, created_at, updated_at, deleted_at
		from manifests
		where id = $1 and deleted_at is null
	`, id))
}

func (s *PostgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := db.DB().ExecContext(ctx, `
		update manifests
		set deleted_at = $2, updated_at = $2
		where id = $1 and deleted_at is null
	`, id, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func scanManifest(scanner interface{ Scan(dest ...any) error }) (*manifestdomain.Manifest, error) {
	var (
		item                manifestdomain.Manifest
		artifactPushedAt    sql.NullTime
		servicesJSON        []byte
		workloadConfigJSON  []byte
		renderedObjectsJSON []byte
		deletedAt           sql.NullTime
	)
	if err := scanner.Scan(
		&item.ID,
		&item.ApplicationID,
		&item.EnvironmentID,
		&item.ImageID,
		&item.ImageRef,
		&item.ArtifactRepository,
		&item.ArtifactTag,
		&item.ArtifactRef,
		&item.ArtifactDigest,
		&item.ArtifactMediaType,
		&artifactPushedAt,
		&servicesJSON,
		&workloadConfigJSON,
		&renderedObjectsJSON,
		&item.RenderedYAML,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	if len(servicesJSON) > 0 {
		if err := json.Unmarshal(servicesJSON, &item.ServicesSnapshot); err != nil {
			return nil, err
		}
	}
	if len(workloadConfigJSON) > 0 {
		if err := json.Unmarshal(workloadConfigJSON, &item.WorkloadConfigSnapshot); err != nil {
			return nil, err
		}
	}
	if len(renderedObjectsJSON) > 0 {
		if err := json.Unmarshal(renderedObjectsJSON, &item.RenderedObjects); err != nil {
			return nil, err
		}
	}
	item.ArtifactPushedAt = dbsql.TimePtrFromNull(artifactPushedAt)
	item.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	return &item, nil
}
