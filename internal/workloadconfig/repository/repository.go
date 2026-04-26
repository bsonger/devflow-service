package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	"github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	"github.com/google/uuid"
)

type ListFilter struct {
	ApplicationID  *uuid.UUID
	EnvironmentID  string
	IncludeDeleted bool
	Name           string
}

type Store interface {
	Create(context.Context, *domain.WorkloadConfig) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*domain.WorkloadConfig, error)
	Update(context.Context, *domain.WorkloadConfig) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, ListFilter) ([]domain.WorkloadConfig, error)
}

type PostgresStore struct{}

func NewPostgresStore() Store {
	return &PostgresStore{}
}

func (s *PostgresStore) Create(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error) {
	_, err := db.Postgres().ExecContext(ctx, `
		insert into workload_configs (
			id, application_id, environment_id, name, description, replicas, service_account_name, resources, probes, env, labels, workload_type, strategy, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`, item.ID, item.ApplicationID, dbsql.EmptyToNull(item.EnvironmentID), item.Name, item.Description, item.Replicas, dbsql.EmptyToNull(item.ServiceAccountName), dbsql.MustMarshalJSON(item.Resources, "[]"), dbsql.MustMarshalJSON(item.Probes, "[]"), dbsql.MustMarshalJSON(item.Env, "[]"), dbsql.MustMarshalJSON(item.Labels, "[]"), item.WorkloadType, item.Strategy, item.CreatedAt, item.UpdatedAt, item.DeletedAt)
	if err != nil {
		return uuid.Nil, err
	}
	return item.ID, nil
}

func (s *PostgresStore) Get(ctx context.Context, id uuid.UUID) (*domain.WorkloadConfig, error) {
	return scanWorkloadConfig(db.Postgres().QueryRowContext(ctx, `
		select id, application_id, environment_id, name, description, replicas, service_account_name, resources, probes, env, labels, workload_type, strategy, created_at, updated_at, deleted_at
		from workload_configs where id=$1 and deleted_at is null
	`, id))
}

func (s *PostgresStore) Update(ctx context.Context, item *domain.WorkloadConfig) error {
	result, err := db.Postgres().ExecContext(ctx, `
		update workload_configs
		set application_id=$2, environment_id=$3, name=$4, description=$5, replicas=$6, service_account_name=$7, resources=$8, probes=$9, env=$10, labels=$11, workload_type=$12, strategy=$13, updated_at=$14
		where id=$1 and deleted_at is null
	`, item.ID, item.ApplicationID, dbsql.EmptyToNull(item.EnvironmentID), item.Name, item.Description, item.Replicas, dbsql.EmptyToNull(item.ServiceAccountName), dbsql.MustMarshalJSON(item.Resources, "[]"), dbsql.MustMarshalJSON(item.Probes, "[]"), dbsql.MustMarshalJSON(item.Env, "[]"), dbsql.MustMarshalJSON(item.Labels, "[]"), item.WorkloadType, item.Strategy, item.UpdatedAt)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result, err := db.Postgres().ExecContext(ctx, `
		update workload_configs set deleted_at=$2, updated_at=$2
		where id=$1 and deleted_at is null
	`, id, now)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) List(ctx context.Context, filter ListFilter) ([]domain.WorkloadConfig, error) {
	query := `
		select id, application_id, environment_id, name, description, replicas, service_account_name, resources, probes, env, labels, workload_type, strategy, created_at, updated_at, deleted_at
		from workload_configs
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
	if filter.EnvironmentID != "" {
		args = append(args, filter.EnvironmentID)
		clauses = append(clauses, dbsql.PlaceholderClause("environment_id", len(args)))
	}
	if filter.Name != "" {
		args = append(args, filter.Name)
		clauses = append(clauses, dbsql.PlaceholderClause("name", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"
	rows, err := db.Postgres().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.WorkloadConfig
	for rows.Next() {
		item, err := scanWorkloadConfig(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func scanWorkloadConfig(scanner interface{ Scan(dest ...any) error }) (*domain.WorkloadConfig, error) {
	var (
		item               domain.WorkloadConfig
		environmentID      sql.NullString
		serviceAccountName sql.NullString
		resourcesJSON      []byte
		probesJSON         []byte
		envJSON            []byte
		labelsJSON         []byte
		deletedAt          sql.NullTime
	)
	if err := scanner.Scan(&item.ID, &item.ApplicationID, &environmentID, &item.Name, &item.Description, &item.Replicas, &serviceAccountName, &resourcesJSON, &probesJSON, &envJSON, &labelsJSON, &item.WorkloadType, &item.Strategy, &item.CreatedAt, &item.UpdatedAt, &deletedAt); err != nil {
		return nil, err
	}
	if environmentID.Valid {
		item.EnvironmentID = environmentID.String
	}
	if serviceAccountName.Valid {
		item.ServiceAccountName = serviceAccountName.String
	}
	if len(resourcesJSON) > 0 {
		_ = json.Unmarshal(resourcesJSON, &item.Resources)
	}
	if len(probesJSON) > 0 {
		_ = json.Unmarshal(probesJSON, &item.Probes)
	}
	if len(envJSON) > 0 {
		_ = json.Unmarshal(envJSON, &item.Env)
	}
	if len(labelsJSON) > 0 {
		_ = json.Unmarshal(labelsJSON, &item.Labels)
	}
	item.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	return &item, nil
}

var _ Store = (*PostgresStore)(nil)
