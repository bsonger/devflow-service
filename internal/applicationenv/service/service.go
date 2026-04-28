package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	appconfigdomain "github.com/bsonger/devflow-service/internal/appconfig/domain"
	appconfigservice "github.com/bsonger/devflow-service/internal/appconfig/service"
	appdomain "github.com/bsonger/devflow-service/internal/application/domain"
	"github.com/bsonger/devflow-service/internal/application/service"
	"github.com/bsonger/devflow-service/internal/applicationenv/domain"
	"github.com/bsonger/devflow-service/internal/applicationenv/repository"
	envdomain "github.com/bsonger/devflow-service/internal/environment/domain"
	envservice "github.com/bsonger/devflow-service/internal/environment/service"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	workloadconfigdomain "github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	workloadconfigservice "github.com/bsonger/devflow-service/internal/workloadconfig/service"
	"github.com/google/uuid"
)

var (
	ErrEnvironmentIDRequired        = sharederrs.Required("environment_id")
	ErrApplicationReferenceNotFound = sharederrs.InvalidArgument("application reference not found")
	ErrEnvironmentReferenceNotFound = sharederrs.InvalidArgument("environment reference not found")
)

type applicationReader interface {
	Get(context.Context, uuid.UUID) (*appdomain.Application, error)
}

type environmentReader interface {
	Get(context.Context, uuid.UUID) (*envdomain.Environment, error)
}

type appConfigReader interface {
	List(context.Context, appconfigservice.AppConfigListFilter) ([]appconfigdomain.AppConfig, error)
}

type workloadConfigReader interface {
	List(context.Context, workloadconfigservice.WorkloadConfigListFilter) ([]workloadconfigdomain.WorkloadConfig, error)
}

type Service interface {
	Attach(context.Context, uuid.UUID, domain.BindingInput) (*domain.Binding, error)
	Get(context.Context, uuid.UUID, string) (*domain.Binding, error)
	List(context.Context, uuid.UUID) ([]BindingView, error)
	Delete(context.Context, uuid.UUID, string) error
	GetDetail(context.Context, uuid.UUID, string) (*BindingDetail, error)
}

type BindingView struct {
	domain.Binding
	Environment *envdomain.Environment `json:"environment,omitempty"`
}

type BindingDetail struct {
	BindingView
	AppConfigs      []appconfigdomain.AppConfig           `json:"app_configs,omitempty"`
	WorkloadConfigs []workloadconfigdomain.WorkloadConfig `json:"workload_configs,omitempty"`
}

type bindingService struct {
	store           repository.Store
	applications    applicationReader
	environments    environmentReader
	appConfigs      appConfigReader
	workloadConfigs workloadConfigReader
}

var DefaultService Service = NewService(
	repository.NewPostgresStore(),
	service.DefaultService,
	envservice.DefaultService,
	appconfigservice.NewAppConfigService(nil),
	workloadconfigservice.NewWorkloadConfigService(),
)

func NewService(
	store repository.Store,
	applications applicationReader,
	environments environmentReader,
	appConfigs appConfigReader,
	workloadConfigs workloadConfigReader,
) Service {
	return &bindingService{
		store:           store,
		applications:    applications,
		environments:    environments,
		appConfigs:      appConfigs,
		workloadConfigs: workloadConfigs,
	}
}

func (s *bindingService) Attach(ctx context.Context, applicationId uuid.UUID, input domain.BindingInput) (*domain.Binding, error) {
	environmentId := strings.TrimSpace(input.EnvironmentID)
	if environmentId == "" {
		return nil, ErrEnvironmentIDRequired
	}
	if err := s.validateReferences(ctx, applicationId, environmentId); err != nil {
		return nil, err
	}

	item := &domain.Binding{
		ApplicationID: applicationId,
		EnvironmentID: environmentId,
	}
	item.WithCreateDefault()
	_, err := s.store.Create(ctx, item)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *bindingService) Get(ctx context.Context, applicationId uuid.UUID, environmentId string) (*domain.Binding, error) {
	return s.store.Get(ctx, applicationId, strings.TrimSpace(environmentId))
}

func (s *bindingService) List(ctx context.Context, applicationId uuid.UUID) ([]BindingView, error) {
	items, err := s.store.ListByApplication(ctx, applicationId)
	if err != nil {
		return nil, err
	}

	out := make([]BindingView, 0, len(items))
	for _, item := range items {
		view := BindingView{Binding: item}
		if env, err := s.lookupEnvironment(ctx, item.EnvironmentID); err == nil {
			view.Environment = env
		}
		out = append(out, view)
	}

	return out, nil
}

func (s *bindingService) Delete(ctx context.Context, applicationId uuid.UUID, environmentId string) error {
	return s.store.Delete(ctx, applicationId, strings.TrimSpace(environmentId))
}

func (s *bindingService) GetDetail(ctx context.Context, applicationId uuid.UUID, environmentId string) (*BindingDetail, error) {
	item, err := s.Get(ctx, applicationId, environmentId)
	if err != nil {
		return nil, err
	}

	view := BindingView{Binding: *item}
	if env, err := s.lookupEnvironment(ctx, item.EnvironmentID); err == nil {
		view.Environment = env
	} else {
		return nil, err
	}

	appConfigs, err := s.resolveAppConfigs(ctx, applicationId, item.EnvironmentID)
	if err != nil {
		return nil, err
	}
	workloadConfigs, err := s.resolveWorkloadConfigs(ctx, applicationId, item.EnvironmentID)
	if err != nil {
		return nil, err
	}

	return &BindingDetail{
		BindingView:     view,
		AppConfigs:      appConfigs,
		WorkloadConfigs: workloadConfigs,
	}, nil
}

func (s *bindingService) validateReferences(ctx context.Context, applicationId uuid.UUID, environmentId string) error {
	if applicationId == uuid.Nil {
		return sharederrs.Required("application_id")
	}
	if s.applications != nil {
		if _, err := s.applications.Get(ctx, applicationId); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrApplicationReferenceNotFound
			}
			return err
		}
	}

	if _, err := s.lookupEnvironment(ctx, environmentId); err != nil {
		return err
	}

	return nil
}

func (s *bindingService) lookupEnvironment(ctx context.Context, environmentId string) (*envdomain.Environment, error) {
	trimmed := strings.TrimSpace(environmentId)
	if trimmed == "" {
		return nil, ErrEnvironmentIDRequired
	}

	id, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, sharederrs.InvalidArgument("environment_id is invalid")
	}
	if s.environments == nil {
		return nil, nil
	}

	item, err := s.environments.Get(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEnvironmentReferenceNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *bindingService) resolveAppConfigs(ctx context.Context, applicationId uuid.UUID, environmentId string) ([]appconfigdomain.AppConfig, error) {
	if s.appConfigs == nil {
		return nil, nil
	}
	return s.appConfigs.List(ctx, appconfigservice.AppConfigListFilter{
		ApplicationID: &applicationId,
		EnvironmentID: environmentId,
	})
}

func (s *bindingService) resolveWorkloadConfigs(ctx context.Context, applicationId uuid.UUID, _ string) ([]workloadconfigdomain.WorkloadConfig, error) {
	if s.workloadConfigs == nil {
		return nil, nil
	}

	return s.workloadConfigs.List(ctx, workloadconfigservice.WorkloadConfigListFilter{
		ApplicationID: &applicationId,
	})
}
