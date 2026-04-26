package service

import (
	"context"
	"strings"

	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	"github.com/bsonger/devflow-service/internal/workloadconfig/repository"
	"github.com/google/uuid"
)

type WorkloadConfigListFilter struct {
	ApplicationID  *uuid.UUID
	IncludeDeleted bool
	Name           string
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
		Name:           filter.Name,
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
	if strings.TrimSpace(item.Name) == "" {
		messages = append(messages, "name is required")
	}
	if item.Replicas < 0 {
		messages = append(messages, "replicas must be >= 0")
	}
	if strings.TrimSpace(item.WorkloadType) == "" {
		messages = append(messages, "workload_type is required")
	}
	switch item.Strategy {
	case "", "canary", "bluegreen", "rolling-update", "rolling":
	default:
		messages = append(messages, "strategy is invalid")
	}
	return sharederrs.JoinInvalid(messages)
}
