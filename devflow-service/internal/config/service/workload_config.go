package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/config/domain"
	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/google/uuid"
)

type WorkloadConfigListFilter struct {
	ApplicationID  *uuid.UUID
	EnvironmentID  string
	IncludeDeleted bool
	Name           string
}

type WorkloadConfigService struct{}

func NewWorkloadConfigService() *WorkloadConfigService {
	return &WorkloadConfigService{}
}

func (s *WorkloadConfigService) Create(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error) {
	if err := validateWorkloadConfig(item); err != nil {
		return uuid.Nil, err
	}
	_, err := db.Postgres().ExecContext(ctx, `
		insert into workload_configs (
			id, application_id, environment_id, name, description, replicas, exposed, resources, probes, env, labels, workload_type, strategy, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`, item.ID, item.ApplicationID, emptyToNull(item.EnvironmentID), item.Name, item.Description, item.Replicas, item.Exposed, marshalJSON(item.Resources), marshalJSON(item.Probes), marshalJSON(item.Env), marshalJSON(item.Labels), item.WorkloadType, item.Strategy, item.CreatedAt, item.UpdatedAt, item.DeletedAt)
	if err != nil {
		return uuid.Nil, err
	}
	return item.ID, nil
}

func (s *WorkloadConfigService) Get(ctx context.Context, id uuid.UUID) (*domain.WorkloadConfig, error) {
	return scanWorkloadConfig(db.Postgres().QueryRowContext(ctx, `
		select id, application_id, environment_id, name, description, replicas, exposed, resources, probes, env, labels, workload_type, strategy, created_at, updated_at, deleted_at
		from workload_configs where id=$1 and deleted_at is null
	`, id))
}

func (s *WorkloadConfigService) Update(ctx context.Context, item *domain.WorkloadConfig) error {
	if err := validateWorkloadConfig(item); err != nil {
		return err
	}
	current, err := s.Get(ctx, item.ID)
	if err != nil {
		return err
	}
	item.CreatedAt = current.CreatedAt
	item.DeletedAt = current.DeletedAt
	item.WithUpdateDefault()
	result, err := db.Postgres().ExecContext(ctx, `
		update workload_configs
		set application_id=$2, environment_id=$3, name=$4, description=$5, replicas=$6, exposed=$7, resources=$8, probes=$9, env=$10, labels=$11, workload_type=$12, strategy=$13, updated_at=$14
		where id=$1 and deleted_at is null
	`, item.ID, item.ApplicationID, emptyToNull(item.EnvironmentID), item.Name, item.Description, item.Replicas, item.Exposed, marshalJSON(item.Resources), marshalJSON(item.Probes), marshalJSON(item.Env), marshalJSON(item.Labels), item.WorkloadType, item.Strategy, item.UpdatedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *WorkloadConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result, err := db.Postgres().ExecContext(ctx, `
		update workload_configs set deleted_at=$2, updated_at=$2
		where id=$1 and deleted_at is null
	`, id, now)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *WorkloadConfigService) List(ctx context.Context, filter WorkloadConfigListFilter) ([]domain.WorkloadConfig, error) {
	query := `
		select id, application_id, environment_id, name, description, replicas, exposed, resources, probes, env, labels, workload_type, strategy, created_at, updated_at, deleted_at
		from workload_configs
	`
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.ApplicationID != nil {
		args = append(args, *filter.ApplicationID)
		clauses = append(clauses, placeholderClause("application_id", len(args)))
	}
	if filter.EnvironmentID != "" {
		args = append(args, filter.EnvironmentID)
		clauses = append(clauses, placeholderClause("environment_id", len(args)))
	}
	if filter.Name != "" {
		args = append(args, filter.Name)
		clauses = append(clauses, placeholderClause("name", len(args)))
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

func validateWorkloadConfig(item *domain.WorkloadConfig) error {
	if item == nil {
		return errors.New("workload_config is required")
	}
	var errs []string
	if item.ApplicationID == uuid.Nil {
		errs = append(errs, "application_id is required")
	}
	if strings.TrimSpace(item.Name) == "" {
		errs = append(errs, "name is required")
	}
	if item.Replicas < 0 {
		errs = append(errs, "replicas must be >= 0")
	}
	if strings.TrimSpace(item.WorkloadType) == "" {
		errs = append(errs, "workload_type is required")
	}
	switch item.Strategy {
	case "", "canary", "bluegreen", "rolling-update", "rolling":
	default:
		errs = append(errs, "strategy is invalid")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func scanWorkloadConfig(scanner interface{ Scan(dest ...any) error }) (*domain.WorkloadConfig, error) {
	var (
		item          domain.WorkloadConfig
		environmentID sql.NullString
		resourcesJSON []byte
		probesJSON    []byte
		envJSON       []byte
		labelsJSON    []byte
		deletedAt     sql.NullTime
	)
	if err := scanner.Scan(&item.ID, &item.ApplicationID, &environmentID, &item.Name, &item.Description, &item.Replicas, &item.Exposed, &resourcesJSON, &probesJSON, &envJSON, &labelsJSON, &item.WorkloadType, &item.Strategy, &item.CreatedAt, &item.UpdatedAt, &deletedAt); err != nil {
		return nil, err
	}
	if environmentID.Valid {
		item.EnvironmentID = environmentID.String
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
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
	return &item, nil
}

func emptyToNull(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
