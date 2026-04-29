package repository

import (
	"context"
	"database/sql"
	"strings"

	"github.com/bsonger/devflow-service/internal/applicationenv/domain"
	platformdb "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	"github.com/google/uuid"
)

type Store interface {
	Create(context.Context, *domain.Binding) (uuid.UUID, error)
	Get(context.Context, uuid.UUID, string) (*domain.Binding, error)
	ListByApplication(context.Context, uuid.UUID) ([]domain.Binding, error)
	Delete(context.Context, uuid.UUID, string) error
}

type postgresStore struct{}

func NewPostgresStore() Store {
	return &postgresStore{}
}

func (s *postgresStore) Create(ctx context.Context, binding *domain.Binding) (uuid.UUID, error) {
	if binding.ID == uuid.Nil {
		binding.ID = uuid.New()
	}

	err := platformdb.Postgres().QueryRowContext(ctx, `
		insert into application_environment_bindings (
			binding_id, application_id, environment_id, created_at, updated_at
		) values ($1,$2,$3,$4,$5)
		on conflict (application_id, environment_id)
		do update set
			updated_at = excluded.updated_at,
			deleted_at = null
		returning binding_id, created_at, updated_at
	`, binding.ID, binding.ApplicationID, binding.EnvironmentID, binding.CreatedAt, binding.UpdatedAt).Scan(&binding.ID, &binding.CreatedAt, &binding.UpdatedAt)
	if err != nil {
		return uuid.Nil, err
	}

	return binding.ID, nil
}

func (s *postgresStore) Get(ctx context.Context, applicationId uuid.UUID, environmentId string) (*domain.Binding, error) {
	return scanBinding(platformdb.Postgres().QueryRowContext(ctx, `
		select binding_id, application_id, environment_id, created_at, updated_at, deleted_at
		from application_environment_bindings
		where application_id = $1 and environment_id = $2 and deleted_at is null
	`, applicationId, strings.TrimSpace(environmentId)))
}

func (s *postgresStore) ListByApplication(ctx context.Context, applicationId uuid.UUID) ([]domain.Binding, error) {
	rows, err := platformdb.Postgres().QueryContext(ctx, `
		select binding_id, application_id, environment_id, created_at, updated_at, deleted_at
		from application_environment_bindings
		where application_id = $1 and deleted_at is null
		order by created_at desc
	`, applicationId)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]domain.Binding, 0)
	for rows.Next() {
		item, err := scanBinding(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}

	return items, rows.Err()
}

func (s *postgresStore) Delete(ctx context.Context, applicationId uuid.UUID, environmentId string) error {
	result, err := platformdb.Postgres().ExecContext(ctx, `
		update application_environment_bindings
		set deleted_at = now(), updated_at = now()
		where application_id = $1 and environment_id = $2 and deleted_at is null
	`, applicationId, strings.TrimSpace(environmentId))
	if err != nil {
		return err
	}

	return dbsql.EnsureRowsAffected(result)
}

func scanBinding(scanner interface{ Scan(dest ...any) error }) (*domain.Binding, error) {
	var (
		item      domain.Binding
		deletedAt sql.NullTime
	)

	if err := scanner.Scan(&item.ID, &item.ApplicationID, &item.EnvironmentID, &item.CreatedAt, &item.UpdatedAt, &deletedAt); err != nil {
		return nil, err
	}

	item.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	return &item, nil
}
