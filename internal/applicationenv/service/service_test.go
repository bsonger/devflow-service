package service

import (
	"context"
	"database/sql"
	"testing"

	appconfigdomain "github.com/bsonger/devflow-service/internal/appconfig/domain"
	appconfigservice "github.com/bsonger/devflow-service/internal/appconfig/service"
	appdomain "github.com/bsonger/devflow-service/internal/application/domain"
	"github.com/bsonger/devflow-service/internal/applicationenv/domain"
	envdomain "github.com/bsonger/devflow-service/internal/environment/domain"
	workloadconfigdomain "github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	workloadconfigservice "github.com/bsonger/devflow-service/internal/workloadconfig/service"
	"github.com/google/uuid"
)

type stubStore struct {
	createFn            func(context.Context, *domain.Binding) (uuid.UUID, error)
	getFn               func(context.Context, uuid.UUID, string) (*domain.Binding, error)
	listByApplicationFn func(context.Context, uuid.UUID) ([]domain.Binding, error)
	deleteFn            func(context.Context, uuid.UUID, string) error
}

func (s stubStore) Create(ctx context.Context, binding *domain.Binding) (uuid.UUID, error) {
	return s.createFn(ctx, binding)
}
func (s stubStore) Get(ctx context.Context, applicationID uuid.UUID, environmentID string) (*domain.Binding, error) {
	return s.getFn(ctx, applicationID, environmentID)
}
func (s stubStore) ListByApplication(ctx context.Context, applicationID uuid.UUID) ([]domain.Binding, error) {
	return s.listByApplicationFn(ctx, applicationID)
}
func (s stubStore) Delete(ctx context.Context, applicationID uuid.UUID, environmentID string) error {
	return s.deleteFn(ctx, applicationID, environmentID)
}

type stubApplicationReader struct {
	getFn func(context.Context, uuid.UUID) (*appdomain.Application, error)
}

func (s stubApplicationReader) Get(ctx context.Context, id uuid.UUID) (*appdomain.Application, error) {
	return s.getFn(ctx, id)
}

type stubEnvironmentReader struct {
	getFn func(context.Context, uuid.UUID) (*envdomain.Environment, error)
}

func (s stubEnvironmentReader) Get(ctx context.Context, id uuid.UUID) (*envdomain.Environment, error) {
	return s.getFn(ctx, id)
}

type stubAppConfigReader struct {
	listFn func(context.Context, appconfigservice.AppConfigListFilter) ([]appconfigdomain.AppConfig, error)
}

func (s stubAppConfigReader) List(ctx context.Context, filter appconfigservice.AppConfigListFilter) ([]appconfigdomain.AppConfig, error) {
	return s.listFn(ctx, filter)
}

type stubWorkloadConfigReader struct {
	listFn func(context.Context, workloadconfigservice.WorkloadConfigListFilter) ([]workloadconfigdomain.WorkloadConfig, error)
}

func (s stubWorkloadConfigReader) List(ctx context.Context, filter workloadconfigservice.WorkloadConfigListFilter) ([]workloadconfigdomain.WorkloadConfig, error) {
	return s.listFn(ctx, filter)
}

func TestAttachValidatesReferences(t *testing.T) {
	applicationID := uuid.New()
	environmentID := uuid.NewString()

	svc := NewService(
		stubStore{
			createFn: func(_ context.Context, binding *domain.Binding) (uuid.UUID, error) {
				return binding.ID, nil
			},
		},
		stubApplicationReader{getFn: func(_ context.Context, id uuid.UUID) (*appdomain.Application, error) {
			if id != applicationID {
				t.Fatalf("unexpected application id %s", id)
			}
			return &appdomain.Application{}, nil
		}},
		stubEnvironmentReader{getFn: func(_ context.Context, id uuid.UUID) (*envdomain.Environment, error) {
			if id.String() != environmentID {
				t.Fatalf("unexpected environment id %s", id)
			}
			return &envdomain.Environment{}, nil
		}},
		nil,
		nil,
	)

	item, err := svc.Attach(context.Background(), applicationID, domain.BindingInput{EnvironmentID: environmentID})
	if err != nil {
		t.Fatal(err)
	}
	if item.ApplicationID != applicationID || item.EnvironmentID != environmentID {
		t.Fatalf("unexpected binding %+v", item)
	}
}

func TestGetDetailFallsBackToBaseConfigs(t *testing.T) {
	applicationID := uuid.New()
	environmentID := uuid.NewString()

	svc := NewService(
		stubStore{
			getFn: func(_ context.Context, appID uuid.UUID, envID string) (*domain.Binding, error) {
				item := &domain.Binding{ApplicationID: appID, EnvironmentID: envID}
				item.WithCreateDefault()
				return item, nil
			},
		},
		nil,
		stubEnvironmentReader{getFn: func(_ context.Context, _ uuid.UUID) (*envdomain.Environment, error) {
			return &envdomain.Environment{Name: "staging"}, nil
		}},
		stubAppConfigReader{listFn: func(_ context.Context, filter appconfigservice.AppConfigListFilter) ([]appconfigdomain.AppConfig, error) {
			if filter.EnvironmentID == environmentID {
				return nil, nil
			}
			if filter.EnvironmentID == BaseEnvironmentID {
				return []appconfigdomain.AppConfig{{Name: "base-config", EnvironmentID: BaseEnvironmentID}}, nil
			}
			return nil, nil
		}},
		stubWorkloadConfigReader{listFn: func(_ context.Context, filter workloadconfigservice.WorkloadConfigListFilter) ([]workloadconfigdomain.WorkloadConfig, error) {
			if filter.EnvironmentID == environmentID {
				return nil, nil
			}
			if filter.EnvironmentID == BaseEnvironmentID {
				return []workloadconfigdomain.WorkloadConfig{{Name: "base-workload", EnvironmentID: BaseEnvironmentID}}, nil
			}
			return nil, nil
		}},
	)

	item, err := svc.GetDetail(context.Background(), applicationID, environmentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(item.AppConfigs) != 1 || item.AppConfigs[0].EnvironmentID != BaseEnvironmentID {
		t.Fatalf("unexpected app configs %+v", item.AppConfigs)
	}
	if len(item.WorkloadConfigs) != 1 || item.WorkloadConfigs[0].EnvironmentID != BaseEnvironmentID {
		t.Fatalf("unexpected workload configs %+v", item.WorkloadConfigs)
	}
}

func TestAttachReturnsReferenceErrors(t *testing.T) {
	svc := NewService(
		stubStore{},
		stubApplicationReader{getFn: func(_ context.Context, _ uuid.UUID) (*appdomain.Application, error) {
			return nil, sql.ErrNoRows
		}},
		nil,
		nil,
		nil,
	)

	_, err := svc.Attach(context.Background(), uuid.New(), domain.BindingInput{EnvironmentID: uuid.NewString()})
	if err != ErrApplicationReferenceNotFound {
		t.Fatalf("got %v want %v", err, ErrApplicationReferenceNotFound)
	}
}
