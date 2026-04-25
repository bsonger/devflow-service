package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

type ListFilter struct {
	IncludeDeleted bool
	ApplicationID  *uuid.UUID
	ManifestID     *uuid.UUID
	ImageID        *uuid.UUID
	Status         string
	Type           string
}

type Store interface {
	Insert(ctx context.Context, release *model.Release) error
	Get(ctx context.Context, id uuid.UUID) (*model.Release, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter ListFilter) ([]*model.Release, error)
	UpdateRow(ctx context.Context, release *model.Release) error
	UpdateSteps(ctx context.Context, release *model.Release) error
}

type PostgresStore struct{}

func NewPostgresStore() Store {
	return &PostgresStore{}
}

func (s *PostgresStore) Insert(ctx context.Context, release *model.Release) error {
	stepsJSON, err := dbsql.MarshalJSON(release.Steps, "[]")
	if err != nil {
		return err
	}
	_, err = db.DB().ExecContext(ctx, `
		insert into releases (
			id, execution_intent_id, application_id, manifest_id, image_id, env, type, steps, status, external_ref, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, release.ID, dbsql.NullableUUIDPtr(release.ExecutionIntentID), release.ApplicationID, release.ManifestID, release.ImageID, release.Env, release.Type, stepsJSON, release.Status, release.ExternalRef, release.CreatedAt, release.UpdatedAt, release.DeletedAt)
	return err
}

func (s *PostgresStore) Get(ctx context.Context, id uuid.UUID) (*model.Release, error) {
	return scanRelease(db.DB().QueryRowContext(ctx, `
		select id, execution_intent_id, application_id, manifest_id, image_id, env, type, steps, status, external_ref, created_at, updated_at, deleted_at
		from releases
		where id = $1 and deleted_at is null
	`, id))
}

func (s *PostgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := db.DB().ExecContext(ctx, `
		update releases
		set deleted_at = $2, updated_at = $2
		where id = $1 and deleted_at is null
	`, id, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) List(ctx context.Context, filter ListFilter) ([]*model.Release, error) {
	query := `
		select id, execution_intent_id, application_id, manifest_id, image_id, env, type, steps, status, external_ref, created_at, updated_at, deleted_at
		from releases
	`
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.ApplicationID != nil {
		args = append(args, *filter.ApplicationID)
		clauses = append(clauses, dbsql.PlaceholderClause("application_id", len(args)))
	}
	if filter.ManifestID != nil {
		args = append(args, *filter.ManifestID)
		clauses = append(clauses, dbsql.PlaceholderClause("manifest_id", len(args)))
	}
	if filter.ImageID != nil {
		args = append(args, *filter.ImageID)
		clauses = append(clauses, dbsql.PlaceholderClause("image_id", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, dbsql.PlaceholderClause("status", len(args)))
	}
	if filter.Type != "" {
		args = append(args, filter.Type)
		clauses = append(clauses, dbsql.PlaceholderClause("type", len(args)))
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
	out := make([]*model.Release, 0)
	for rows.Next() {
		item, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) UpdateRow(ctx context.Context, release *model.Release) error {
	stepsJSON, err := dbsql.MarshalJSON(release.Steps, "[]")
	if err != nil {
		return err
	}
	result, err := db.DB().ExecContext(ctx, `
		update releases
		set execution_intent_id=$2, application_id=$3, manifest_id=$4, image_id=$5, env=$6, type=$7, steps=$8, status=$9, external_ref=$10, updated_at=$11, deleted_at=$12
		where id = $1
	`, release.ID, dbsql.NullableUUIDPtr(release.ExecutionIntentID), release.ApplicationID, release.ManifestID, release.ImageID, release.Env, release.Type, stepsJSON, release.Status, release.ExternalRef, release.UpdatedAt, release.DeletedAt)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) UpdateSteps(ctx context.Context, release *model.Release) error {
	stepsJSON, err := dbsql.MarshalJSON(release.Steps, "[]")
	if err != nil {
		return err
	}
	result, err := db.DB().ExecContext(ctx, `
		update releases
		set steps = $2, updated_at = $3
		where id = $1 and deleted_at is null
	`, release.ID, stepsJSON, release.UpdatedAt)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func scanRelease(scanner interface{ Scan(dest ...any) error }) (*model.Release, error) {
	var (
		item            model.Release
		executionIntent sql.NullString
		stepsBytes      []byte
		deletedAt       sql.NullTime
	)

	if err := scanner.Scan(
		&item.ID,
		&executionIntent,
		&item.ApplicationID,
		&item.ManifestID,
		&item.ImageID,
		&item.Env,
		&item.Type,
		&stepsBytes,
		&item.Status,
		&item.ExternalRef,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}

	var err error
	item.ExecutionIntentID, err = dbsql.ParseNullUUID(executionIntent)
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
