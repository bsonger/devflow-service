package service

import (
	"context"

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
	return sharederrs.JoinInvalid(messages)
}
