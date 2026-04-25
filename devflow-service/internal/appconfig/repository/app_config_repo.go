package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/appconfig/domain"
	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/google/uuid"
)

type AppConfigStore interface {
	Create(ctx context.Context, cfg *domain.AppConfig) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.AppConfig, error)
	Update(ctx context.Context, cfg *domain.AppConfig) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter AppConfigListFilter) ([]domain.AppConfig, error)
}

type AppConfigListFilter struct {
	ApplicationID  *uuid.UUID
	EnvironmentID  string
	IncludeDeleted bool
	Name           string
}

type appConfigPostgresStore struct{}

func NewAppConfigPostgresStore() AppConfigStore {
	return &appConfigPostgresStore{}
}

func (s *appConfigPostgresStore) Create(ctx context.Context, cfg *domain.AppConfig) (uuid.UUID, error) {
	if err := validateAppConfig(cfg); err != nil {
		return uuid.Nil, err
	}
	if strings.TrimSpace(cfg.SourcePath) == "" {
		cfg.SourcePath = deriveAppConfigSourcePath(cfg.Name)
	}
	cfg.MountPath = normalizeAppConfigMountPath(cfg.MountPath)
	_, err := db.Postgres().ExecContext(ctx, `
		insert into configurations (
			id, application_id, name, env, description, format, data, mount_path, labels, source_path, files, latest_revision_no, latest_revision_id, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'[]'::jsonb,$11,$12,$13,$14,$15)
	`, cfg.ID, cfg.ApplicationID, cfg.Name, cfg.EnvironmentID, cfg.Description, cfg.Format, cfg.Data, cfg.MountPath, marshalJSON(cfg.Labels), cfg.SourcePath, cfg.LatestRevisionNo, nullableUUIDPtr(cfg.LatestRevisionID), cfg.CreatedAt, cfg.UpdatedAt, cfg.DeletedAt)
	if err != nil {
		return uuid.Nil, err
	}
	return cfg.ID, nil
}

func (s *appConfigPostgresStore) Get(ctx context.Context, id uuid.UUID) (*domain.AppConfig, error) {
	return scanAppConfig(db.Postgres().QueryRowContext(ctx, `
		select id, application_id, name, env, description, format, data, mount_path, labels, source_path, latest_revision_no, latest_revision_id, created_at, updated_at, deleted_at
		from configurations where id=$1 and deleted_at is null
	`, id))
}

func (s *appConfigPostgresStore) Update(ctx context.Context, cfg *domain.AppConfig) error {
	if err := validateAppConfig(cfg); err != nil {
		return err
	}
	current, err := s.Get(ctx, cfg.ID)
	if err != nil {
		return err
	}
	cfg.CreatedAt = current.CreatedAt
	cfg.DeletedAt = current.DeletedAt
	if strings.TrimSpace(cfg.SourcePath) == "" {
		cfg.SourcePath = current.SourcePath
	}
	cfg.MountPath = normalizeAppConfigMountPath(cfg.MountPath)
	cfg.WithUpdateDefault()
	result, err := db.Postgres().ExecContext(ctx, `
		update configurations
		set application_id=$2, name=$3, env=$4, description=$5, format=$6, data=$7, mount_path=$8, labels=$9, source_path=$10, updated_at=$11
		where id=$1 and deleted_at is null
	`, cfg.ID, cfg.ApplicationID, cfg.Name, cfg.EnvironmentID, cfg.Description, cfg.Format, cfg.Data, cfg.MountPath, marshalJSON(cfg.Labels), cfg.SourcePath, cfg.UpdatedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *appConfigPostgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result, err := db.Postgres().ExecContext(ctx, `
		update configurations set deleted_at=$2, updated_at=$2
		where id=$1 and deleted_at is null
	`, id, now)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *appConfigPostgresStore) List(ctx context.Context, filter AppConfigListFilter) ([]domain.AppConfig, error) {
	query := `
		select id, application_id, name, env, description, format, data, mount_path, labels, source_path, latest_revision_no, latest_revision_id, created_at, updated_at, deleted_at
		from configurations
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
		clauses = append(clauses, placeholderClause("env", len(args)))
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
	items := make([]domain.AppConfig, 0)
	for rows.Next() {
		item, err := scanAppConfig(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func validateAppConfig(cfg *domain.AppConfig) error {
	if cfg == nil {
		return errors.New("app_config is required")
	}
	if errs := validateAppConfigInput(cfg.ApplicationID, cfg.EnvironmentID); len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return errors.New("name is required")
	}
	cfg.MountPath = normalizeAppConfigMountPath(cfg.MountPath)
	return nil
}

func validateAppConfigInput(applicationID uuid.UUID, environmentID string) []string {
	var errs []string
	if applicationID == uuid.Nil {
		errs = append(errs, "application_id is required")
	}
	if strings.TrimSpace(environmentID) == "" {
		errs = append(errs, "environment_id is required")
	}
	return errs
}

func deriveAppConfigSourcePath(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	return fmt.Sprintf("applications/devflow-platform/services/%s", trimmed)
}

func normalizeAppConfigMountPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "/etc/devflow/config"
	}
	return trimmed
}

func scanAppConfig(scanner interface{ Scan(dest ...any) error }) (*domain.AppConfig, error) {
	var (
		cfg              domain.AppConfig
		applicationID    sql.NullString
		labelsJSON       []byte
		latestRevisionID sql.NullString
		deletedAt        sql.NullTime
	)
	if err := scanner.Scan(&cfg.ID, &applicationID, &cfg.Name, &cfg.EnvironmentID, &cfg.Description, &cfg.Format, &cfg.Data, &cfg.MountPath, &labelsJSON, &cfg.SourcePath, &cfg.LatestRevisionNo, &latestRevisionID, &cfg.CreatedAt, &cfg.UpdatedAt, &deletedAt); err != nil {
		return nil, err
	}
	if applicationID.Valid {
		parsed, err := uuid.Parse(applicationID.String)
		if err != nil {
			return nil, err
		}
		cfg.ApplicationID = parsed
	}
	if latestRevisionID.Valid {
		parsed, err := uuid.Parse(latestRevisionID.String)
		if err != nil {
			return nil, err
		}
		cfg.LatestRevisionID = &parsed
	}
	if len(labelsJSON) > 0 {
		_ = json.Unmarshal(labelsJSON, &cfg.Labels)
	}
	cfg.MountPath = normalizeAppConfigMountPath(cfg.MountPath)
	if deletedAt.Valid {
		cfg.DeletedAt = &deletedAt.Time
	}
	return &cfg, nil
}

func ensureRowsAffected(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func placeholderClause(column string, index int) string {
	return fmt.Sprintf("%s=$%d", column, index)
}

func marshalJSON(value any) []byte {
	if value == nil {
		return []byte("[]")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return []byte("[]")
	}
	return payload
}

func nullableUUIDPtr(id *uuid.UUID) any {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	return *id
}
