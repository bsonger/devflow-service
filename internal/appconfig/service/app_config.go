package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/appconfig/domain"
	appconfigrepo "github.com/bsonger/devflow-service/internal/appconfig/repository"
	"github.com/bsonger/devflow-service/internal/platform/configrepo"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
)

var ErrConfigSourceNotFound = sharederrs.FailedPrecondition("configuration source path not found")
var ErrConfigRepositoryUnavailable = sharederrs.FailedPrecondition("configuration repository is not configured")
var ErrConfigRepositorySyncFailed = sharederrs.FailedPrecondition("configuration repository sync failed")

type AppConfigListFilter struct {
	ApplicationID  *uuid.UUID
	EnvironmentID  string
	IncludeDeleted bool
	Name           string
}

type AppConfigSyncResult struct {
	Revision *domain.AppConfigRevision
	Created  bool
}

type appConfigRepository interface {
	ReadSnapshot(ctx context.Context, sourcePath, env string) (*configrepo.Snapshot, error)
}

type AppConfigService struct {
	repo                appConfigRepository
	store               appconfigrepo.AppConfigStore
	environmentResolver environmentResolver
}

func NewAppConfigService(repo appConfigRepository) *AppConfigService {
	return &AppConfigService{
		repo:  repo,
		store: appconfigrepo.NewAppConfigPostgresStore(),
	}
}

func (s *AppConfigService) WithEnvironmentResolver(resolver environmentResolver) *AppConfigService {
	s.environmentResolver = resolver
	return s
}

func (s *AppConfigService) Create(ctx context.Context, cfg *domain.AppConfig) (uuid.UUID, error) {
	if err := validateAppConfig(cfg); err != nil {
		return uuid.Nil, err
	}
	if strings.TrimSpace(cfg.SourcePath) == "" {
		cfg.SourcePath = deriveAppConfigSourcePath(cfg.Name)
	}
	cfg.MountPath = normalizeAppConfigMountPath(cfg.MountPath)
	return s.store.Create(ctx, cfg)
}

func (s *AppConfigService) Get(ctx context.Context, id uuid.UUID) (*domain.AppConfig, error) {
	cfg, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if cfg.LatestRevisionID == nil || *cfg.LatestRevisionID == uuid.Nil {
		return cfg, nil
	}
	revision, err := s.getRevision(ctx, *cfg.LatestRevisionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return cfg, nil
		}
		return nil, err
	}
	cfg.Files = revision.Files
	cfg.RenderedConfigMap = revision.RenderedConfigMap
	cfg.SourceCommit = revision.SourceCommit
	return cfg, nil
}

func (s *AppConfigService) Update(ctx context.Context, cfg *domain.AppConfig) error {
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
	return s.store.Update(ctx, cfg)
}

func (s *AppConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.Delete(ctx, id)
}

func (s *AppConfigService) List(ctx context.Context, filter AppConfigListFilter) ([]domain.AppConfig, error) {
	return s.store.List(ctx, appconfigrepo.AppConfigListFilter{
		ApplicationID:  filter.ApplicationID,
		EnvironmentID:  filter.EnvironmentID,
		IncludeDeleted: filter.IncludeDeleted,
		Name:           filter.Name,
	})
}

func (s *AppConfigService) Sync(ctx context.Context, id uuid.UUID) (*AppConfigSyncResult, error) {
	if s.repo == nil {
		return nil, ErrConfigRepositoryUnavailable
	}
	cfg, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if namedSnapshot, resolvedPath, resolvedErr := s.readEnvironmentNamedSnapshot(ctx, cfg); resolvedErr == nil {
		snapshot := namedSnapshot
		if resolvedPath != "" && resolvedPath != cfg.SourcePath {
			cfg.SourcePath = resolvedPath
			if updateErr := s.updateSourcePath(ctx, cfg.ID, resolvedPath); updateErr != nil {
				return nil, updateErr
			}
		}
		return s.syncWithSnapshot(ctx, cfg, snapshot)
	}
	snapshot, err := s.repo.ReadSnapshot(ctx, cfg.SourcePath, cfg.EnvironmentID)
	if err != nil {
		if errors.Is(err, configrepo.ErrSourcePathNotFound) {
			if namedSnapshot, resolvedPath, resolvedErr := s.readEnvironmentNamedSnapshot(ctx, cfg); resolvedErr == nil {
				snapshot = namedSnapshot
				if resolvedPath != "" && resolvedPath != cfg.SourcePath {
					cfg.SourcePath = resolvedPath
					if updateErr := s.updateSourcePath(ctx, cfg.ID, resolvedPath); updateErr != nil {
						return nil, updateErr
					}
				}
				err = nil
			} else {
				fallbackPath := deriveAppConfigSourcePath(cfg.Name)
				if fallbackPath != "" && fallbackPath != cfg.SourcePath {
					snapshot, err = s.repo.ReadSnapshot(ctx, fallbackPath, cfg.EnvironmentID)
					if err == nil {
						cfg.SourcePath = fallbackPath
						if updateErr := s.updateSourcePath(ctx, cfg.ID, fallbackPath); updateErr != nil {
							return nil, updateErr
						}
					}
				}
			}
		}
		if err != nil {
			if errors.Is(err, configrepo.ErrSourcePathNotFound) {
				return nil, ErrConfigSourceNotFound
			}
			if errors.Is(err, configrepo.ErrRepositorySyncFailed) {
				return nil, fmt.Errorf("%w: %v", ErrConfigRepositorySyncFailed, err)
			}
			return nil, err
		}
	}
	return s.syncWithSnapshot(ctx, cfg, snapshot)
}

func (s *AppConfigService) syncWithSnapshot(ctx context.Context, cfg *domain.AppConfig, snapshot *configrepo.Snapshot) (*AppConfigSyncResult, error) {
	latest, err := s.getLatestRevision(ctx, cfg.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if latest != nil && latest.SourceDigest == snapshot.SourceDigest {
		return &AppConfigSyncResult{Revision: latest, Created: false}, nil
	}
	revisionNo := 1
	if latest != nil {
		revisionNo = latest.RevisionNo + 1
	}
	revision := &domain.AppConfigRevision{
		ID:                uuid.New(),
		AppConfigID:       cfg.ID,
		RevisionNo:        revisionNo,
		Files:             snapshotFilesToDomainFiles(snapshot.Files),
		RenderedConfigMap: renderConfigMap(snapshot.Files),
		ContentHash:       snapshot.SourceDigest,
		SourceCommit:      snapshot.SourceCommit,
		SourceDigest:      snapshot.SourceDigest,
		CreatedAt:         time.Now().Format(time.RFC3339),
	}
	if err := s.insertRevision(ctx, revision); err != nil {
		return nil, err
	}
	if err := s.updateLatestRevision(ctx, cfg, revision); err != nil {
		return nil, err
	}
	return &AppConfigSyncResult{Revision: revision, Created: true}, nil
}

func (s *AppConfigService) getLatestRevision(ctx context.Context, appConfigID uuid.UUID) (*domain.AppConfigRevision, error) {
	return s.store.GetLatestRevision(ctx, appConfigID)
}

func (s *AppConfigService) getRevision(ctx context.Context, id uuid.UUID) (*domain.AppConfigRevision, error) {
	return s.store.GetRevision(ctx, id)
}

func (s *AppConfigService) insertRevision(ctx context.Context, revision *domain.AppConfigRevision) error {
	return s.store.InsertRevision(ctx, revision)
}

func (s *AppConfigService) updateLatestRevision(ctx context.Context, cfg *domain.AppConfig, revision *domain.AppConfigRevision) error {
	cfg.LatestRevisionNo = revision.RevisionNo
	cfg.LatestRevisionID = &revision.ID
	cfg.WithUpdateDefault()
	return s.store.UpdateLatestRevision(ctx, cfg.ID, cfg.LatestRevisionNo, revision.ID, cfg.UpdatedAt)
}

func (s *AppConfigService) updateSourcePath(ctx context.Context, id uuid.UUID, sourcePath string) error {
	return s.store.UpdateSourcePath(ctx, id, sourcePath, time.Now())
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

func (s *AppConfigService) readEnvironmentNamedSnapshot(ctx context.Context, cfg *domain.AppConfig) (*configrepo.Snapshot, string, error) {
	if s == nil || s.environmentResolver == nil || cfg == nil {
		return nil, "", configrepo.ErrSourcePathNotFound
	}
	trimmedEnvID := strings.TrimSpace(cfg.EnvironmentID)
	if trimmedEnvID == "" || strings.EqualFold(trimmedEnvID, "base") {
		return nil, "", configrepo.ErrSourcePathNotFound
	}
	environmentName, err := s.environmentResolver.ResolveName(ctx, trimmedEnvID)
	if err != nil {
		return nil, "", err
	}
	resolvedPath := strings.TrimRight(strings.TrimSpace(cfg.SourcePath), "/") + "/" + strings.TrimSpace(environmentName)
	if strings.TrimSpace(resolvedPath) == "" || resolvedPath == "/" {
		return nil, "", configrepo.ErrSourcePathNotFound
	}
	snapshot, err := s.repo.ReadSnapshot(ctx, resolvedPath, environmentName)
	if err != nil {
		return nil, "", err
	}
	return snapshot, resolvedPath, nil
}

func renderConfigMap(files []configrepo.File) domain.RenderedConfigMap {
	data := make(map[string]string, len(files))
	for _, file := range files {
		data[file.Name] = file.Content
	}
	return domain.RenderedConfigMap{Data: data}
}

func snapshotFilesToDomainFiles(files []configrepo.File) []domain.File {
	if len(files) == 0 {
		return nil
	}
	out := make([]domain.File, 0, len(files))
	for _, file := range files {
		out = append(out, domain.File{
			Name:    file.Name,
			Content: file.Content,
		})
	}
	return out
}
