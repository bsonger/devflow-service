package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	"github.com/bsonger/devflow-service/internal/workloadconfig/repository"
	"github.com/google/uuid"
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
	if err := validateWorkloadConfig(item); err != nil {
		return uuid.Nil, err
	}
	existing, err := s.store.List(ctx, repository.ListFilter{ApplicationID: &item.ApplicationID})
	if err != nil {
		return uuid.Nil, err
	}
	if len(existing) > 0 {
		return uuid.Nil, sharederrs.Conflict("workload config already exists for application")
	}
	return s.store.Create(ctx, item)
}

func (s *WorkloadConfigService) Get(ctx context.Context, id uuid.UUID) (*domain.WorkloadConfig, error) {
	return s.store.Get(ctx, id)
}

func (s *WorkloadConfigService) Update(ctx context.Context, item *domain.WorkloadConfig) error {
	if err := validateWorkloadConfig(item); err != nil {
		return err
	}
	current, err := s.Get(ctx, item.ID)
	if err != nil {
		return err
	}
	if current.ApplicationID != item.ApplicationID {
		return sharederrs.InvalidArgument("application_id cannot be changed")
	}
	item.CreatedAt = current.CreatedAt
	item.DeletedAt = current.DeletedAt
	item.WithUpdateDefault()
	return s.store.Update(ctx, item)
}

func (s *WorkloadConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.Delete(ctx, id)
}

func (s *WorkloadConfigService) List(ctx context.Context, filter WorkloadConfigListFilter) ([]domain.WorkloadConfig, error) {
	return s.store.List(ctx, repository.ListFilter{
		ApplicationID:  filter.ApplicationID,
		IncludeDeleted: filter.IncludeDeleted,
	})
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
