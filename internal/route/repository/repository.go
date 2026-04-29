package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	"github.com/bsonger/devflow-service/internal/route/domain"
	"github.com/google/uuid"
)

type ListFilter struct {
	ApplicationID  uuid.UUID
	EnvironmentID  string
	IncludeDeleted bool
	Name           string
}

type Store interface {
	Create(ctx context.Context, route *domain.Route) (uuid.UUID, error)
	Get(ctx context.Context, applicationId, id uuid.UUID) (*domain.Route, error)
	Update(ctx context.Context, route *domain.Route) error
	Delete(ctx context.Context, applicationId, id uuid.UUID) error
	List(ctx context.Context, filter ListFilter) ([]domain.Route, error)
}

type PostgresStore struct{}

func NewPostgresStore() Store {
	return &PostgresStore{}
}

func (s *PostgresStore) Create(ctx context.Context, item *domain.Route) (uuid.UUID, error) {
	_, err := db.Postgres().ExecContext(ctx, `
		insert into routes (
			id, application_id, environment_id, name, host, path, service_name, service_port, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, item.ID, dbsql.NullableUUID(item.ApplicationID), item.EnvironmentID, item.Name, item.Host, item.Path, item.ServiceName, item.ServicePort, item.CreatedAt, item.UpdatedAt, item.DeletedAt)
	if err != nil {
		return uuid.Nil, err
	}
	return item.ID, nil
}

func (s *PostgresStore) Get(ctx context.Context, applicationId, id uuid.UUID) (*domain.Route, error) {
	return scanRoute(db.Postgres().QueryRowContext(ctx, `
		select id, application_id, environment_id, name, host, path, service_name, service_port, created_at, updated_at, deleted_at
		from routes
		where application_id = $1 and id = $2 and deleted_at is null
	`, applicationId, id))
}

func (s *PostgresStore) Update(ctx context.Context, item *domain.Route) error {
	current, err := s.Get(ctx, item.ApplicationID, item.ID)
	if err != nil {
		return err
	}
	item.CreatedAt = current.CreatedAt
	item.DeletedAt = current.DeletedAt
	item.WithUpdateDefault()
	result, err := db.Postgres().ExecContext(ctx, `
		update routes
		set environment_id=$3, name=$4, host=$5, path=$6, service_name=$7, service_port=$8, updated_at=$9
		where application_id=$1 and id=$2 and deleted_at is null
	`, item.ApplicationID, item.ID, item.EnvironmentID, item.Name, item.Host, item.Path, item.ServiceName, item.ServicePort, item.UpdatedAt)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) Delete(ctx context.Context, applicationId, id uuid.UUID) error {
	now := time.Now()
	result, err := db.Postgres().ExecContext(ctx, `
		update routes set deleted_at=$3, updated_at=$3
		where application_id=$1 and id=$2 and deleted_at is null
	`, applicationId, id, now)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) List(ctx context.Context, filter ListFilter) ([]domain.Route, error) {
	query := `
		select id, application_id, environment_id, name, host, path, service_name, service_port, created_at, updated_at, deleted_at
		from routes
	`
	clauses := []string{"application_id = $1"}
	args := []any{filter.ApplicationID}
	if filter.EnvironmentID != "" {
		args = append(args, filter.EnvironmentID)
		clauses = append(clauses, dbsql.PlaceholderClause("environment_id", len(args)))
	}
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.Name != "" {
		args = append(args, filter.Name)
		clauses = append(clauses, dbsql.PlaceholderClause("name", len(args)))
	}
	query += " where " + strings.Join(clauses, " and ") + " order by created_at desc"
	rows, err := db.Postgres().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var items []domain.Route
	for rows.Next() {
		item, err := scanRoute(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func scanRoute(scanner interface{ Scan(dest ...any) error }) (*domain.Route, error) {
	var (
		item          domain.Route
		applicationId sql.NullString
		environmentId sql.NullString
		deletedAt     sql.NullTime
	)
	if err := scanner.Scan(&item.ID, &applicationId, &environmentId, &item.Name, &item.Host, &item.Path, &item.ServiceName, &item.ServicePort, &item.CreatedAt, &item.UpdatedAt, &deletedAt); err != nil {
		return nil, err
	}
	applicationUUID, err := dbsql.ParseNullUUID(applicationId)
	if err != nil {
		return nil, err
	}
	if applicationUUID != nil {
		item.ApplicationID = *applicationUUID
	}
	item.EnvironmentID = environmentId.String
	item.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	return &item, nil
}
