package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

type ListFilter struct {
	IncludeDeleted bool
	ApplicationID  *uuid.UUID
	PipelineID     string
	Status         string
	Branch         string
	Name           string
}

type Store interface {
	Insert(ctx context.Context, image *imagedomain.Image) error
	Get(ctx context.Context, id uuid.UUID) (*imagedomain.Image, error)
	List(ctx context.Context, filter ListFilter) ([]imagedomain.Image, error)
	GetByPipelineID(ctx context.Context, pipelineID string) (*imagedomain.Image, error)
	AssignPipelineID(ctx context.Context, imageID uuid.UUID, pipelineID string) error
	UpdateStatusAndSteps(ctx context.Context, id uuid.UUID, status model.ImageStatus, steps []model.ImageTask, pipelineID string) error
	UpdatePipelineAndSteps(ctx context.Context, id uuid.UUID, pipelineID string, steps []model.ImageTask) error
	UpdateRow(ctx context.Context, image *imagedomain.Image) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type PostgresStore struct{}

func NewPostgresStore() Store {
	return &PostgresStore{}
}

func (s *PostgresStore) Insert(ctx context.Context, m *imagedomain.Image) error {
	stepsJSON, err := dbsql.MarshalJSON(m.Steps, "[]")
	if err != nil {
		return err
	}
	_, err = db.DB().ExecContext(ctx, `
		insert into images (
			id, execution_intent_id, application_id, configuration_revision_id, runtime_spec_revision_id, name, tag, branch, repo_address, commit_hash, digest, pipeline_id, steps, status, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`, m.ID, dbsql.NullableUUIDPtr(m.ExecutionIntentID), m.ApplicationID, dbsql.NullableUUIDPtr(m.ConfigurationRevisionID), dbsql.NullableUUIDPtr(m.RuntimeSpecRevisionID), m.Name, m.Tag, m.Branch, m.RepoAddress, m.CommitHash, m.Digest, m.PipelineID, stepsJSON, m.Status, m.CreatedAt, m.UpdatedAt, m.DeletedAt)
	return err
}

func (s *PostgresStore) Get(ctx context.Context, id uuid.UUID) (*imagedomain.Image, error) {
	return scanImage(db.DB().QueryRowContext(ctx, `
		select id, execution_intent_id, application_id, configuration_revision_id, runtime_spec_revision_id, name, tag, branch, repo_address, commit_hash, digest, pipeline_id, steps, status, created_at, updated_at, deleted_at
		from images
		where id = $1 and deleted_at is null
	`, id))
}

func (s *PostgresStore) List(ctx context.Context, filter ListFilter) ([]imagedomain.Image, error) {
	query := `
		select id, execution_intent_id, application_id, configuration_revision_id, runtime_spec_revision_id, name, tag, branch, repo_address, commit_hash, digest, pipeline_id, steps, status, created_at, updated_at, deleted_at
		from images
	`
	clauses := make([]string, 0, 6)
	args := make([]any, 0, 6)
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.ApplicationID != nil {
		args = append(args, *filter.ApplicationID)
		clauses = append(clauses, dbsql.PlaceholderClause("application_id", len(args)))
	}
	if filter.PipelineID != "" {
		args = append(args, filter.PipelineID)
		clauses = append(clauses, dbsql.PlaceholderClause("pipeline_id", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, dbsql.PlaceholderClause("status", len(args)))
	}
	if filter.Branch != "" {
		args = append(args, filter.Branch)
		clauses = append(clauses, dbsql.PlaceholderClause("branch", len(args)))
	}
	if filter.Name != "" {
		args = append(args, filter.Name)
		clauses = append(clauses, dbsql.PlaceholderClause("name", len(args)))
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
	out := make([]imagedomain.Image, 0)
	for rows.Next() {
		item, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) GetByPipelineID(ctx context.Context, pipelineID string) (*imagedomain.Image, error) {
	return scanImage(db.DB().QueryRowContext(ctx, `
		select id, execution_intent_id, application_id, configuration_revision_id, runtime_spec_revision_id, name, tag, branch, repo_address, commit_hash, digest, pipeline_id, steps, status, created_at, updated_at, deleted_at
		from images
		where pipeline_id = $1 and deleted_at is null
	`, pipelineID))
}

func (s *PostgresStore) AssignPipelineID(ctx context.Context, imageID uuid.UUID, pipelineID string) error {
	result, err := db.DB().ExecContext(ctx, `
		update images
		set pipeline_id = $2, updated_at = $3
		where id = $1 and deleted_at is null
	`, imageID, pipelineID, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) UpdateStatusAndSteps(ctx context.Context, id uuid.UUID, status model.ImageStatus, steps []model.ImageTask, pipelineID string) error {
	stepsJSON, err := dbsql.MarshalJSON(steps, "[]")
	if err != nil {
		return err
	}
	result, err := db.DB().ExecContext(ctx, `
		update images
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
		update images
		set pipeline_id = $2, steps = $3, updated_at = $4
		where id = $1 and deleted_at is null
	`, id, pipelineID, stepsJSON, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) UpdateRow(ctx context.Context, m *imagedomain.Image) error {
	stepsJSON, err := dbsql.MarshalJSON(m.Steps, "[]")
	if err != nil {
		return err
	}
	result, err := db.DB().ExecContext(ctx, `
		update images
		set execution_intent_id=$2, application_id=$3, configuration_revision_id=$4, runtime_spec_revision_id=$5, name=$6, tag=$7, branch=$8, repo_address=$9, commit_hash=$10, digest=$11, pipeline_id=$12, steps=$13, status=$14, updated_at=$15, deleted_at=$16
		where id=$1 and deleted_at is null
	`, m.ID, dbsql.NullableUUIDPtr(m.ExecutionIntentID), m.ApplicationID, dbsql.NullableUUIDPtr(m.ConfigurationRevisionID), dbsql.NullableUUIDPtr(m.RuntimeSpecRevisionID), m.Name, m.Tag, m.Branch, m.RepoAddress, m.CommitHash, m.Digest, m.PipelineID, stepsJSON, m.Status, m.UpdatedAt, m.DeletedAt)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := db.DB().ExecContext(ctx, `
		update images
		set deleted_at = $2, updated_at = $2
		where id = $1 and deleted_at is null
	`, id, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func scanImage(scanner interface{ Scan(dest ...any) error }) (*imagedomain.Image, error) {
	var (
		item                    imagedomain.Image
		executionIntent         sql.NullString
		configurationRevisionID sql.NullString
		runtimeSpecRevisionID   sql.NullString
		stepsBytes              []byte
		deletedAt               sql.NullTime
	)

	if err := scanner.Scan(
		&item.ID,
		&executionIntent,
		&item.ApplicationID,
		&configurationRevisionID,
		&runtimeSpecRevisionID,
		&item.Name,
		&item.Tag,
		&item.Branch,
		&item.RepoAddress,
		&item.CommitHash,
		&item.Digest,
		&item.PipelineID,
		&stepsBytes,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}

	intentID, err := dbsql.ParseNullUUID(executionIntent)
	if err != nil {
		return nil, err
	}
	item.ExecutionIntentID = intentID
	item.ConfigurationRevisionID, err = dbsql.ParseNullUUID(configurationRevisionID)
	if err != nil {
		return nil, err
	}
	item.RuntimeSpecRevisionID, err = dbsql.ParseNullUUID(runtimeSpecRevisionID)
	if err != nil {
		return nil, err
	}
	if len(stepsBytes) > 0 {
		if err := json.Unmarshal(stepsBytes, &item.Steps); err != nil {
			return nil, err
		}
	}
	item.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	return &item, nil
}
