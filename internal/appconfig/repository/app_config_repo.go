package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/appconfig/domain"
	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
)

type AppConfigStore interface {
	Create(ctx context.Context, cfg *domain.AppConfig) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.AppConfig, error)
	Update(ctx context.Context, cfg *domain.AppConfig) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter AppConfigListFilter) ([]domain.AppConfig, error)
	GetLatestRevision(ctx context.Context, appConfigID uuid.UUID) (*domain.AppConfigRevision, error)
	GetRevision(ctx context.Context, id uuid.UUID) (*domain.AppConfigRevision, error)
	InsertRevision(ctx context.Context, revision *domain.AppConfigRevision) error
	UpdateLatestRevision(ctx context.Context, configID uuid.UUID, revisionNo int, revisionID uuid.UUID, updatedAt time.Time) error
	UpdateSourcePath(ctx context.Context, id uuid.UUID, sourcePath string, updatedAt time.Time) error
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
	`, cfg.ID, cfg.ApplicationID, cfg.Name, cfg.EnvironmentID, cfg.Description, cfg.Format, cfg.Data, cfg.MountPath, dbsql.MustMarshalJSON(cfg.Labels, "[]"), cfg.SourcePath, cfg.LatestRevisionNo, dbsql.NullableUUIDPtr(cfg.LatestRevisionID), cfg.CreatedAt, cfg.UpdatedAt, cfg.DeletedAt)
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
	`, cfg.ID, cfg.ApplicationID, cfg.Name, cfg.EnvironmentID, cfg.Description, cfg.Format, cfg.Data, cfg.MountPath, dbsql.MustMarshalJSON(cfg.Labels, "[]"), cfg.SourcePath, cfg.UpdatedAt)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
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
	return dbsql.EnsureRowsAffected(result)
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
		clauses = append(clauses, dbsql.PlaceholderClause("application_id", len(args)))
	}
	if filter.EnvironmentID != "" {
		args = append(args, filter.EnvironmentID)
		clauses = append(clauses, dbsql.PlaceholderClause("env", len(args)))
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

func (s *appConfigPostgresStore) GetLatestRevision(ctx context.Context, appConfigID uuid.UUID) (*domain.AppConfigRevision, error) {
	return scanAppConfigRevision(db.Postgres().QueryRowContext(ctx, `
		select id, configuration_id, revision_no, files, rendered_configmap, content_hash, source_commit, source_digest, created_at
		from configuration_revisions
		where configuration_id=$1
		order by revision_no desc limit 1
	`, appConfigID))
}

func (s *appConfigPostgresStore) GetRevision(ctx context.Context, id uuid.UUID) (*domain.AppConfigRevision, error) {
	return scanAppConfigRevision(db.Postgres().QueryRowContext(ctx, `
		select id, configuration_id, revision_no, files, rendered_configmap, content_hash, source_commit, source_digest, created_at
		from configuration_revisions
		where id=$1
	`, id))
}

func (s *appConfigPostgresStore) InsertRevision(ctx context.Context, revision *domain.AppConfigRevision) error {
	filesJSON, err := json.Marshal(revision.Files)
	if err != nil {
		return err
	}
	renderedJSON, err := json.Marshal(revision.RenderedConfigMap)
	if err != nil {
		return err
	}
	createdAt, err := time.Parse(time.RFC3339, revision.CreatedAt)
	if err != nil {
		return err
	}
	_, err = db.Postgres().ExecContext(ctx, `
		insert into configuration_revisions (
			id, configuration_id, revision_no, files, rendered_configmap, content_hash, source_commit, source_digest, message, created_by, created_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,'','',$9)
	`, revision.ID, revision.AppConfigID, revision.RevisionNo, filesJSON, renderedJSON, revision.ContentHash, revision.SourceCommit, revision.SourceDigest, createdAt)
	return err
}

func (s *appConfigPostgresStore) UpdateLatestRevision(ctx context.Context, configID uuid.UUID, revisionNo int, revisionID uuid.UUID, updatedAt time.Time) error {
	_, err := db.Postgres().ExecContext(ctx, `
		update configurations
		set latest_revision_no=$2, latest_revision_id=$3, updated_at=$4
		where id=$1 and deleted_at is null
	`, configID, revisionNo, revisionID, updatedAt)
	return err
}

func (s *appConfigPostgresStore) UpdateSourcePath(ctx context.Context, id uuid.UUID, sourcePath string, updatedAt time.Time) error {
	_, err := db.Postgres().ExecContext(ctx, `
		update configurations
		set source_path=$2, updated_at=$3
		where id=$1 and deleted_at is null
	`, id, sourcePath, updatedAt)
	return err
}

func validateAppConfig(cfg *domain.AppConfig) error {
	if cfg == nil {
		return sharederrs.Required("app_config")
	}
	if messages := validateAppConfigInput(cfg.ApplicationID, cfg.EnvironmentID); len(messages) > 0 {
		return sharederrs.JoinInvalid(messages)
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return sharederrs.Required("name")
	}
	cfg.MountPath = normalizeAppConfigMountPath(cfg.MountPath)
	return nil
}

func validateAppConfigInput(applicationId uuid.UUID, environmentId string) []string {
	var errs []string
	if applicationId == uuid.Nil {
		errs = append(errs, "application_id is required")
	}
	if strings.TrimSpace(environmentId) == "" {
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
		applicationId    sql.NullString
		labelsJSON       []byte
		latestRevisionID sql.NullString
		deletedAt        sql.NullTime
	)
	if err := scanner.Scan(&cfg.ID, &applicationId, &cfg.Name, &cfg.EnvironmentID, &cfg.Description, &cfg.Format, &cfg.Data, &cfg.MountPath, &labelsJSON, &cfg.SourcePath, &cfg.LatestRevisionNo, &latestRevisionID, &cfg.CreatedAt, &cfg.UpdatedAt, &deletedAt); err != nil {
		return nil, err
	}
	applicationUUID, err := dbsql.ParseNullUUID(applicationId)
	if err != nil {
		return nil, err
	}
	if applicationUUID != nil {
		cfg.ApplicationID = *applicationUUID
	}
	cfg.LatestRevisionID, err = dbsql.ParseNullUUID(latestRevisionID)
	if err != nil {
		return nil, err
	}
	if len(labelsJSON) > 0 {
		_ = json.Unmarshal(labelsJSON, &cfg.Labels)
	}
	cfg.MountPath = normalizeAppConfigMountPath(cfg.MountPath)
	cfg.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	return &cfg, nil
}

func scanAppConfigRevision(scanner interface{ Scan(dest ...any) error }) (*domain.AppConfigRevision, error) {
	var (
		revision     domain.AppConfigRevision
		filesJSON    []byte
		renderedJSON []byte
		createdAt    time.Time
	)
	if err := scanner.Scan(&revision.ID, &revision.AppConfigID, &revision.RevisionNo, &filesJSON, &renderedJSON, &revision.ContentHash, &revision.SourceCommit, &revision.SourceDigest, &createdAt); err != nil {
		return nil, err
	}
	if len(filesJSON) > 0 {
		if err := json.Unmarshal(filesJSON, &revision.Files); err != nil {
			return nil, err
		}
	}
	if len(renderedJSON) > 0 {
		if err := json.Unmarshal(renderedJSON, &revision.RenderedConfigMap); err != nil {
			return nil, err
		}
	}
	revision.CreatedAt = createdAt.Format(time.RFC3339)
	return &revision, nil
}
