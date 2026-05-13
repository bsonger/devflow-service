package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/appconfig/domain"
	appconfigrepo "github.com/bsonger/devflow-service/internal/appconfig/repository"
	environmentservice "github.com/bsonger/devflow-service/internal/environment/service"
	"github.com/bsonger/devflow-service/internal/platform/configrepo"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

var ErrConfigSourceNotFound = sharederrs.FailedPrecondition("configuration source path not found")
var ErrConfigRepositoryUnavailable = sharederrs.FailedPrecondition("configuration repository is not configured")
var ErrConfigRepositorySyncFailed = sharederrs.FailedPrecondition("configuration repository sync failed")
var ErrConfigObservabilityBoundary = sharederrs.FailedPrecondition("configuration repository contains forbidden observability fields")

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
	log := platformobs.OperationLogger(ctx, "config_service", "create_app_config", "app_config",
		zap.String("devflow.application.id", appConfigApplicationID(cfg)),
		zap.String("devflow.environment.id", appConfigEnvironmentID(cfg)),
	)
	if err := validateAppConfig(cfg); err != nil {
		platformobs.LogOperationFailure(log, "create app config failed", err)
		return uuid.Nil, err
	}
	cfg.MountPath = normalizeAppConfigMountPath(cfg.MountPath)
	id, err := s.store.Create(ctx, cfg)
	if err != nil {
		platformobs.LogOperationFailure(log, "create app config failed", err)
		return uuid.Nil, err
	}
	platformobs.LogOperationSuccess(log, "app config created",
		zap.String("resource_id", id.String()),
	)
	return id, nil
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
	log := platformobs.OperationLogger(ctx, "config_service", "update_app_config", "app_config",
		zap.String("resource_id", appConfigID(cfg)),
		zap.String("devflow.application.id", appConfigApplicationID(cfg)),
		zap.String("devflow.environment.id", appConfigEnvironmentID(cfg)),
	)
	if err := validateAppConfig(cfg); err != nil {
		platformobs.LogOperationFailure(log, "update app config failed", err)
		return err
	}
	current, err := s.Get(ctx, cfg.ID)
	if err != nil {
		platformobs.LogOperationFailure(log, "update app config failed", err)
		return err
	}
	cfg.CreatedAt = current.CreatedAt
	cfg.DeletedAt = current.DeletedAt
	cfg.SourceDirectory = current.SourceDirectory
	cfg.MountPath = normalizeAppConfigMountPath(cfg.MountPath)
	if err := s.store.Update(ctx, cfg); err != nil {
		platformobs.LogOperationFailure(log, "update app config failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "app config updated")
	return nil
}

func (s *AppConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	log := platformobs.OperationLogger(ctx, "config_service", "delete_app_config", "app_config",
		zap.String("resource_id", id.String()),
	)
	if err := s.store.Delete(ctx, id); err != nil {
		platformobs.LogOperationFailure(log, "delete app config failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "app config deleted")
	return nil
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
	log := platformobs.OperationLogger(ctx, "config_service", "sync_app_config", "app_config",
		zap.String("resource_id", id.String()),
	)
	if s.repo == nil {
		platformobs.LogOperationFailure(log, "sync app config failed", ErrConfigRepositoryUnavailable)
		return nil, ErrConfigRepositoryUnavailable
	}
	cfg, err := s.Get(ctx, id)
	if err != nil {
		platformobs.LogOperationFailure(log, "sync app config failed", err)
		return nil, err
	}
	sourceDirectory, err := s.deriveSourceDirectory(ctx, cfg)
	if err != nil {
		platformobs.LogOperationFailure(log, "sync app config failed", err)
		return nil, err
	}
	snapshot, err := s.repo.ReadSnapshot(ctx, sourceDirectory, "")
	if err != nil {
		if errors.Is(err, configrepo.ErrSourcePathNotFound) {
			platformobs.LogOperationFailure(log, "sync app config failed", ErrConfigSourceNotFound)
			return nil, ErrConfigSourceNotFound
		}
		if errors.Is(err, configrepo.ErrRepositorySyncFailed) {
			wrapped := fmt.Errorf("%w: %v", ErrConfigRepositorySyncFailed, err)
			platformobs.LogOperationFailure(log, "sync app config failed", wrapped)
			return nil, wrapped
		}
		platformobs.LogOperationFailure(log, "sync app config failed", err)
		return nil, err
	}
	if strings.TrimSpace(cfg.SourceDirectory) != strings.TrimSpace(sourceDirectory) {
		cfg.SourceDirectory = sourceDirectory
		if updateErr := s.updateSourceDirectory(ctx, cfg.ID, sourceDirectory); updateErr != nil {
			platformobs.LogOperationFailure(log, "sync app config failed", updateErr)
			return nil, updateErr
		}
	}
	result, err := s.syncWithSnapshot(ctx, cfg, snapshot)
	if err != nil {
		platformobs.LogOperationFailure(log, "sync app config failed", err)
		return nil, err
	}
	platformobs.LogOperationSuccess(log, "app config synced",
		zap.Bool("revision_created", result != nil && result.Created),
	)
	return result, nil
}

func (s *AppConfigService) syncWithSnapshot(ctx context.Context, cfg *domain.AppConfig, snapshot *configrepo.Snapshot) (*AppConfigSyncResult, error) {
	if err := validateSnapshotObservabilityBoundary(snapshot); err != nil {
		return nil, err
	}
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

func appConfigID(cfg *domain.AppConfig) string {
	if cfg == nil || cfg.ID == uuid.Nil {
		return ""
	}
	return cfg.ID.String()
}

func appConfigApplicationID(cfg *domain.AppConfig) string {
	if cfg == nil || cfg.ApplicationID == uuid.Nil {
		return ""
	}
	return cfg.ApplicationID.String()
}

func appConfigEnvironmentID(cfg *domain.AppConfig) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.EnvironmentID)
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

func validateSnapshotObservabilityBoundary(snapshot *configrepo.Snapshot) error {
	if snapshot == nil {
		return nil
	}
	var messages []string
	for _, file := range snapshot.Files {
		messages = append(messages, validateObservabilityBoundaryFile(file)...)
	}
	if len(messages) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrConfigObservabilityBoundary, strings.Join(messages, "; "))
}

func validateObservabilityBoundaryFile(file configrepo.File) []string {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(file.Name)))
	if ext != ".yaml" && ext != ".yml" {
		return nil
	}
	var decoded map[string]any
	if err := yaml.Unmarshal([]byte(file.Content), &decoded); err != nil {
		return nil
	}
	otelNode, ok := decoded["otel"]
	if !ok {
		return nil
	}
	otel, ok := toStringMap(otelNode)
	if !ok {
		return nil
	}
	var messages []string
	if value := strings.TrimSpace(stringMapValue(otel, "service_name")); value != "" {
		messages = append(messages, fmt.Sprintf("%s forbids otel.service_name; use OTEL_SERVICE_NAME env var", file.Name))
	}
	if value := strings.TrimSpace(stringMapValue(otel, "resource_attributes")); value != "" {
		messages = append(messages, fmt.Sprintf("%s forbids otel.resource_attributes=%q; use Deployment env vars for service.namespace/service.version/deployment.environment.name", file.Name, value))
	}
	return messages
}

func toStringMap(value any) (map[string]any, bool) {
	switch current := value.(type) {
	case map[string]any:
		return current, true
	case map[any]any:
		out := make(map[string]any, len(current))
		for key, item := range current {
			text, ok := key.(string)
			if !ok {
				continue
			}
			out[text] = item
		}
		return out, true
	default:
		return nil, false
	}
}

func stringMapValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	raw, ok := values[key]
	if !ok || raw == nil {
		return ""
	}
	switch current := raw.(type) {
	case string:
		return current
	default:
		return fmt.Sprintf("%v", current)
	}
}
