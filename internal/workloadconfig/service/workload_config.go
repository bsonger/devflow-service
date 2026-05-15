package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	"github.com/bsonger/devflow-service/internal/workloadconfig/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type WorkloadConfigListFilter struct {
	ApplicationID  *uuid.UUID
	IncludeDeleted bool
}

type WorkloadConfigService struct {
	store repository.Store
}

func NewWorkloadConfigService() *WorkloadConfigService {
	return &WorkloadConfigService{store: repository.NewPostgresStore()}
}

func (s *WorkloadConfigService) Create(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error) {
	log := platformobs.OperationLogger(ctx, "config_service", "create_workload_config", "workload_config",
		zap.String("devflow.application.id", workloadConfigApplicationID(item)),
	)
	if err := validateWorkloadConfig(item); err != nil {
		platformobs.LogOperationFailure(log, "create workload config failed", err)
		return uuid.Nil, err
	}
	existing, err := s.store.List(ctx, repository.ListFilter{ApplicationID: &item.ApplicationID})
	if err != nil {
		platformobs.LogOperationFailure(log, "create workload config failed", err)
		return uuid.Nil, err
	}
	if len(existing) > 0 {
		err := sharederrs.Conflict("workload config already exists for application")
		platformobs.LogOperationFailure(log, "create workload config failed", err)
		return uuid.Nil, err
	}
	id, err := s.store.Create(ctx, item)
	if err != nil {
		platformobs.LogOperationFailure(log, "create workload config failed", err)
		return uuid.Nil, err
	}
	platformobs.LogOperationSuccess(log, "workload config created",
		zap.String("resource_id", id.String()),
	)
	return id, nil
}

func (s *WorkloadConfigService) Get(ctx context.Context, id uuid.UUID) (*domain.WorkloadConfig, error) {
	return s.store.Get(ctx, id)
}

func (s *WorkloadConfigService) Update(ctx context.Context, item *domain.WorkloadConfig) error {
	log := platformobs.OperationLogger(ctx, "config_service", "update_workload_config", "workload_config",
		zap.String("resource_id", workloadConfigID(item)),
		zap.String("devflow.application.id", workloadConfigApplicationID(item)),
	)
	if err := validateWorkloadConfig(item); err != nil {
		platformobs.LogOperationFailure(log, "update workload config failed", err)
		return err
	}
	current, err := s.Get(ctx, item.ID)
	if err != nil {
		platformobs.LogOperationFailure(log, "update workload config failed", err)
		return err
	}
	if current.ApplicationID != item.ApplicationID {
		err := sharederrs.InvalidArgument("application_id cannot be changed")
		platformobs.LogOperationFailure(log, "update workload config failed", err)
		return err
	}
	item.CreatedAt = current.CreatedAt
	item.DeletedAt = current.DeletedAt
	item.WithUpdateDefault()
	if err := s.store.Update(ctx, item); err != nil {
		platformobs.LogOperationFailure(log, "update workload config failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "workload config updated")
	return nil
}

func (s *WorkloadConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	log := platformobs.OperationLogger(ctx, "config_service", "delete_workload_config", "workload_config",
		zap.String("resource_id", id.String()),
	)
	if err := s.store.Delete(ctx, id); err != nil {
		platformobs.LogOperationFailure(log, "delete workload config failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "workload config deleted")
	return nil
}

func (s *WorkloadConfigService) List(ctx context.Context, filter WorkloadConfigListFilter) ([]domain.WorkloadConfig, error) {
	return s.store.List(ctx, repository.ListFilter{
		ApplicationID:  filter.ApplicationID,
		IncludeDeleted: filter.IncludeDeleted,
	})
}

func workloadConfigID(item *domain.WorkloadConfig) string {
	if item == nil || item.ID == uuid.Nil {
		return ""
	}
	return item.ID.String()
}

func workloadConfigApplicationID(item *domain.WorkloadConfig) string {
	if item == nil || item.ApplicationID == uuid.Nil {
		return ""
	}
	return item.ApplicationID.String()
}

func validateWorkloadConfig(item *domain.WorkloadConfig) error {
	if item == nil {
		return sharederrs.Required("workload_config")
	}
	var messages []string
	if item.ApplicationID == uuid.Nil {
		messages = append(messages, "application_id is required")
	}
	if item.Replicas < 0 {
		messages = append(messages, "replicas must be >= 0")
	}
	messages = append(messages, validateWorkloadResources(item.Resources)...)
	messages = append(messages, validateWorkloadProbes(item.Probes)...)
	messages = append(messages, validateWorkloadMetrics(&item.Metrics)...)
	messages = append(messages, validateWorkloadEmptyDirs(item.EmptyDirs)...)
	messages = append(messages, validateWorkloadEnv(item.Env)...)
	return sharederrs.JoinInvalid(messages)
}

func validateWorkloadResources(resources domain.WorkloadResourceRequirements) []string {
	var messages []string
	if _, ok := domain.WorkloadSizeClassResources[resources.SizeClass]; !ok {
		messages = append(messages, fmt.Sprintf("resources.size_class must be one of: %s", strings.Join(validWorkloadSizeClasses(), ", ")))
	}
	if hasWorkloadResourceList(resources.Requests) {
		messages = append(messages, "resources.requests must not be provided on write")
	}
	if hasWorkloadResourceList(resources.Limits) {
		messages = append(messages, "resources.limits must not be provided on write")
	}
	return messages
}

func validateWorkloadProbes(probes domain.WorkloadProbes) []string {
	var messages []string
	messages = append(messages, validateWorkloadProbe("probes.liveness", probes.Liveness)...)
	messages = append(messages, validateWorkloadProbe("probes.readiness", probes.Readiness)...)
	messages = append(messages, validateWorkloadProbe("probes.startup", probes.Startup)...)
	return messages
}

func validateWorkloadProbe(prefix string, probe *domain.WorkloadProbe) []string {
	if probe == nil {
		return nil
	}
	var messages []string
	path := strings.TrimSpace(probe.Path)
	port := strings.TrimSpace(probe.Port)
	if path != "" {
		if !strings.HasPrefix(path, "/") {
			messages = append(messages, prefix+".path must start with '/'")
		}
		if port == "" {
			messages = append(messages, prefix+".port is required when "+prefix+".path is set")
		}
	}
	return messages
}

func validateWorkloadEnv(env []domain.EnvVar) []string {
	var messages []string
	seen := map[string]int{}
	for i, entry := range env {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			messages = append(messages, fmt.Sprintf("env[%d].name is required", i))
			continue
		}
		if first, ok := seen[name]; ok {
			messages = append(messages, fmt.Sprintf("env[%d].name duplicates env[%d].name %q", i, first, name))
			continue
		}
		seen[name] = i
	}
	return messages
}

func validateWorkloadEmptyDirs(emptyDirs []domain.WorkloadEmptyDir) []string {
	var messages []string
	seenNames := map[string]int{}
	seenMountPaths := map[string]int{}
	for i, item := range emptyDirs {
		name := strings.TrimSpace(item.Name)
		mountPath := strings.TrimSpace(item.MountPath)
		medium := strings.TrimSpace(item.Medium)
		if name == "" {
			messages = append(messages, fmt.Sprintf("empty_dirs[%d].name is required", i))
		} else if first, ok := seenNames[name]; ok {
			messages = append(messages, fmt.Sprintf("empty_dirs[%d].name duplicates empty_dirs[%d].name %q", i, first, name))
		} else {
			seenNames[name] = i
		}
		if mountPath == "" {
			messages = append(messages, fmt.Sprintf("empty_dirs[%d].mount_path is required", i))
		} else {
			if !strings.HasPrefix(mountPath, "/") {
				messages = append(messages, fmt.Sprintf("empty_dirs[%d].mount_path must start with '/'", i))
			}
			if first, ok := seenMountPaths[mountPath]; ok {
				messages = append(messages, fmt.Sprintf("empty_dirs[%d].mount_path duplicates empty_dirs[%d].mount_path %q", i, first, mountPath))
			} else {
				seenMountPaths[mountPath] = i
			}
		}
		if medium != "" && medium != "Memory" {
			messages = append(messages, fmt.Sprintf("empty_dirs[%d].medium must be empty or 'Memory'", i))
		}
	}
	return messages
}

func validateWorkloadMetrics(metrics *domain.WorkloadMetrics) []string {
	if metrics == nil {
		return nil
	}
	if !metrics.Enabled {
		*metrics = domain.WorkloadMetrics{}
		return nil
	}

	var messages []string
	if metrics.Port <= 0 {
		messages = append(messages, "metrics.port must be > 0 when metrics.enabled is true")
	}
	switch metrics.ScrapeProfile {
	case "":
		metrics.ScrapeProfile = domain.WorkloadMetricsScrapeProfileDefault
	case domain.WorkloadMetricsScrapeProfileDefault, domain.WorkloadMetricsScrapeProfileFast, domain.WorkloadMetricsScrapeProfileSlow:
	default:
		messages = append(messages, fmt.Sprintf("metrics.scrape_profile must be one of: %s", strings.Join(validWorkloadMetricsScrapeProfiles(), ", ")))
	}
	return messages
}

func hasWorkloadResourceList(resources domain.WorkloadResourceList) bool {
	return strings.TrimSpace(resources.CPU) != "" || strings.TrimSpace(resources.Memory) != ""
}

func validWorkloadSizeClasses() []string {
	values := make([]string, 0, len(domain.WorkloadSizeClassResources))
	for sizeClass := range domain.WorkloadSizeClassResources {
		values = append(values, string(sizeClass))
	}
	sort.Strings(values)
	return values
}

func validWorkloadMetricsScrapeProfiles() []string {
	values := []string{
		string(domain.WorkloadMetricsScrapeProfileDefault),
		string(domain.WorkloadMetricsScrapeProfileFast),
		string(domain.WorkloadMetricsScrapeProfileSlow),
	}
	sort.Strings(values)
	return values
}
