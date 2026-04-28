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
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

type Store interface {
	Insert(ctx context.Context, manifest *manifestdomain.Manifest) error
	List(ctx context.Context, filter manifestdomain.ManifestListFilter) ([]manifestdomain.Manifest, error)
	Get(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error)
	GetByPipelineID(ctx context.Context, pipelineID string) (*manifestdomain.Manifest, error)
	AssignPipelineID(ctx context.Context, manifestID uuid.UUID, pipelineID string) error
	UpdateStatusAndSteps(ctx context.Context, id uuid.UUID, status model.ManifestStatus, steps []model.ImageTask, pipelineID string) error
	UpdatePipelineAndSteps(ctx context.Context, id uuid.UUID, pipelineID string, steps []model.ImageTask) error
	UpdateBuildResult(ctx context.Context, id uuid.UUID, commitHash, imageRef, imageTag, imageDigest string, status model.ManifestStatus) error
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
	stepsJSON, err := dbsql.MarshalJSON(m.Steps, "[]")
	if err != nil {
		return err
	}
	_, err = db.DB().ExecContext(ctx, `
		insert into manifests (
			id, application_id, git_revision, repo_address, commit_hash, image_ref, image_tag, image_digest, pipeline_id, trace_id, span_id, steps,
			services_snapshot, workload_config_snapshot,
			status, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
	`, m.ID, m.ApplicationID, m.GitRevision, m.RepoAddress, m.CommitHash, m.ImageRef, m.ImageTag, m.ImageDigest, m.PipelineID, m.TraceID, m.SpanID, stepsJSON,
		servicesJSON, workloadJSON,
		m.Status, m.CreatedAt, m.UpdatedAt, m.DeletedAt)
	return err
}

func (s *PostgresStore) List(ctx context.Context, filter manifestdomain.ManifestListFilter) ([]manifestdomain.Manifest, error) {
	query := `
		select id, application_id, git_revision, repo_address, commit_hash, image_ref, image_tag, image_digest, pipeline_id, trace_id, span_id, steps,
			services_snapshot, workload_config_snapshot,
			status, created_at, updated_at, deleted_at
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
		select id, application_id, git_revision, repo_address, commit_hash, image_ref, image_tag, image_digest, pipeline_id, trace_id, span_id, steps,
			services_snapshot, workload_config_snapshot,
			status, created_at, updated_at, deleted_at
		from manifests
		where id = $1 and deleted_at is null
	`, id))
}

func (s *PostgresStore) GetByPipelineID(ctx context.Context, pipelineID string) (*manifestdomain.Manifest, error) {
	return scanManifest(db.DB().QueryRowContext(ctx, `
		select id, application_id, git_revision, repo_address, commit_hash, image_ref, image_tag, image_digest, pipeline_id, trace_id, span_id, steps,
			services_snapshot, workload_config_snapshot,
			status, created_at, updated_at, deleted_at
		from manifests
		where pipeline_id = $1 and deleted_at is null
	`, pipelineID))
}

func (s *PostgresStore) AssignPipelineID(ctx context.Context, manifestID uuid.UUID, pipelineID string) error {
	result, err := db.DB().ExecContext(ctx, `
		update manifests
		set pipeline_id = $2, updated_at = $3
		where id = $1 and deleted_at is null
	`, manifestID, pipelineID, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) UpdateStatusAndSteps(ctx context.Context, id uuid.UUID, status model.ManifestStatus, steps []model.ImageTask, pipelineID string) error {
	stepsJSON, err := dbsql.MarshalJSON(steps, "[]")
	if err != nil {
		return err
	}
	result, err := db.DB().ExecContext(ctx, `
		update manifests
		set status = $2, steps = $3, pipeline_id = $4, updated_at = $5
		where id = $1 and deleted_at is null
	`, id, status, stepsJSON, pipelineID, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) UpdatePipelineAndSteps(ctx context.Context, id uuid.UUID, pipelineID string, steps []model.ImageTask) error {
	stepsJSON, err := dbsql.MarshalJSON(steps, "[]")
	if err != nil {
		return err
	}
	result, err := db.DB().ExecContext(ctx, `
		update manifests
		set pipeline_id = $2, steps = $3, updated_at = $4
		where id = $1 and deleted_at is null
	`, id, pipelineID, stepsJSON, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) UpdateBuildResult(ctx context.Context, id uuid.UUID, commitHash, imageRef, imageTag, imageDigest string, status model.ManifestStatus) error {
	result, err := db.DB().ExecContext(ctx, `
		update manifests
		set commit_hash = $2,
		    image_ref = $3,
		    image_tag = $4,
		    image_digest = $5,
		    status = $6,
		    updated_at = $7
		where id = $1 and deleted_at is null
	`, id, commitHash, imageRef, imageTag, imageDigest, status, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
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
		item               manifestdomain.Manifest
		stepsJSON          []byte
		servicesJSON       []byte
		workloadConfigJSON []byte
		deletedAt          sql.NullTime
	)
	if err := scanner.Scan(
		&item.ID,
		&item.ApplicationID,
		&item.GitRevision,
		&item.RepoAddress,
		&item.CommitHash,
		&item.ImageRef,
		&item.ImageTag,
		&item.ImageDigest,
		&item.PipelineID,
		&item.TraceID,
		&item.SpanID,
		&stepsJSON,
		&servicesJSON,
		&workloadConfigJSON,
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
	if len(stepsJSON) > 0 {
		if err := json.Unmarshal(stepsJSON, &item.Steps); err != nil {
			return nil, err
		}
	}
	if len(workloadConfigJSON) > 0 {
		if err := json.Unmarshal(workloadConfigJSON, &item.WorkloadConfigSnapshot); err != nil {
			return nil, err
		}
	}
	item.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	return &item, nil
}
