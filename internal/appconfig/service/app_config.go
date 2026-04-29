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
	environmentservice "github.com/bsonger/devflow-service/internal/environment/service"
	"github.com/bsonger/devflow-service/internal/platform/configrepo"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
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
}

type AppConfigSyncResult struct {
	Revision *domain.AppConfigRevision
	Created  bool
}

type appConfigRepository interface {
	ReadSnapshot(ctx context.Context, sourcePath, env string) (*configrepo.Snapshot, error)
}

type applicationProjectionReader interface {
	Get(ctx context.Context, id uuid.UUID) (*releasesupport.ApplicationProjection, error)
}

type environmentNameResolver interface {
	ResolveName(ctx context.Context, environmentId string) (string, error)
}

type localEnvironmentResolver struct{}

func (localEnvironmentResolver) ResolveName(ctx context.Context, environmentId string) (string, error) {
	id, err := uuid.Parse(strings.TrimSpace(environmentId))
	if err != nil {
		return "", sharederrs.InvalidArgument("environment_id is invalid")
	}
	environment, err := environmentservice.DefaultService.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if environment == nil || strings.TrimSpace(environment.Name) == "" {
		return "", sharederrs.FailedPrecondition("environment name is empty")
	}
	return strings.TrimSpace(environment.Name), nil
}

type AppConfigService struct {
	repo                appConfigRepository
	store               appconfigrepo.AppConfigStore
	applications        applicationProjectionReader
	environmentResolver environmentNameResolver
}

func NewAppConfigService(repo appConfigRepository) *AppConfigService {
	return &AppConfigService{
		repo:                repo,
		store:               appconfigrepo.NewAppConfigPostgresStore(),
		applications:        releasesupport.ApplicationService,
		environmentResolver: localEnvironmentResolver{},
	}
}

func (s *AppConfigService) WithEnvironmentResolver(resolver environmentNameResolver) *AppConfigService {
	s.environmentResolver = resolver
	return s
}

func (s *AppConfigService) Create(ctx context.Context, cfg *domain.AppConfig) (uuid.UUID, error) {
	if err := validateAppConfig(cfg); err != nil {
		return uuid.Nil, err
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
	cfg.SourceDirectory = current.SourceDirectory
	cfg.MountPath = normalizeAppConfigMountPath(cfg.MountPath)
	return s.store.Update(ctx, cfg)
}

func (s *AppConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.Delete(ctx, id)
}

func (s *AppConfigService) List(ctx context.Context, filter AppConfigListFilter) ([]domain.AppConfig, error) {
	items, err := s.store.List(ctx, appconfigrepo.AppConfigListFilter{
		ApplicationID:  filter.ApplicationID,
		EnvironmentID:  filter.EnvironmentID,
		IncludeDeleted: filter.IncludeDeleted,
	})
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].LatestRevisionID == nil || *items[i].LatestRevisionID == uuid.Nil {
			continue
		}
		revision, err := s.getRevision(ctx, *items[i].LatestRevisionID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		items[i].Files = revision.Files
		items[i].SourceCommit = revision.SourceCommit
	}
	return items, nil
}

func (s *AppConfigService) Sync(ctx context.Context, id uuid.UUID) (*AppConfigSyncResult, error) {
	if s.repo == nil {
		return nil, ErrConfigRepositoryUnavailable
	}
	cfg, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	sourceDirectory, err := s.deriveSourceDirectory(ctx, cfg)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.repo.ReadSnapshot(ctx, sourceDirectory, "")
	if err != nil {
		if errors.Is(err, configrepo.ErrSourcePathNotFound) {
			return nil, ErrConfigSourceNotFound
		}
		if errors.Is(err, configrepo.ErrRepositorySyncFailed) {
			return nil, fmt.Errorf("%w: %v", ErrConfigRepositorySyncFailed, err)
		}
		return nil, err
	}
	if strings.TrimSpace(cfg.SourceDirectory) != strings.TrimSpace(sourceDirectory) {
		cfg.SourceDirectory = sourceDirectory
		if updateErr := s.updateSourceDirectory(ctx, cfg.ID, sourceDirectory); updateErr != nil {
			return nil, updateErr
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
		ID:           uuid.New(),
		AppConfigID:  cfg.ID,
		RevisionNo:   revisionNo,
		Files:        snapshotFilesToDomainFiles(snapshot.Files),
		ContentHash:  snapshot.SourceDigest,
		SourceCommit: snapshot.SourceCommit,
		SourceDigest: snapshot.SourceDigest,
		CreatedAt:    time.Now().Format(time.RFC3339),
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

func (s *AppConfigService) updateSourceDirectory(ctx context.Context, id uuid.UUID, sourceDirectory string) error {
	return s.store.UpdateSourceDirectory(ctx, id, sourceDirectory, time.Now())
}

func validateAppConfig(cfg *domain.AppConfig) error {
	if cfg == nil {
		return sharederrs.Required("app_config")
	}
	if messages := validateAppConfigInput(cfg.ApplicationID, cfg.EnvironmentID); len(messages) > 0 {
		return sharederrs.JoinInvalid(messages)
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

func normalizeAppConfigMountPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "/etc/config"
	}
	return trimmed
}

func (s *AppConfigService) deriveSourceDirectory(ctx context.Context, cfg *domain.AppConfig) (string, error) {
	if s == nil || cfg == nil {
		return "", sharederrs.Required("app_config")
	}
	if s.applications == nil {
		return "", sharederrs.FailedPrecondition("application metadata reader is not configured")
	}
	application, err := s.applications.Get(ctx, cfg.ApplicationID)
	if err != nil {
		return "", err
	}
	if application == nil {
		return "", sharederrs.FailedPrecondition("application metadata is missing")
	}
	projectName := strings.TrimSpace(application.ProjectName)
	applicationName := strings.TrimSpace(application.Name)
	if projectName == "" || applicationName == "" {
		return "", sharederrs.FailedPrecondition("application metadata is incomplete")
	}
	environmentName := strings.TrimSpace(cfg.EnvironmentID)
	if s.environmentResolver != nil {
		resolvedName, err := s.environmentResolver.ResolveName(ctx, cfg.EnvironmentID)
		if err != nil {
			return "", err
		}
		environmentName = strings.TrimSpace(resolvedName)
	}
	if environmentName == "" {
		return "", sharederrs.Required("environment_id")
	}
	return fmt.Sprintf("%s/%s/%s", projectName, applicationName, environmentName), nil
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
