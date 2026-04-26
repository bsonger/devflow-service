package service

import (
	"context"
	"strings"

	"github.com/bsonger/devflow-service/internal/approute/domain"
	"github.com/bsonger/devflow-service/internal/approute/repository"
	appservice "github.com/bsonger/devflow-service/internal/appservice/service"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
)

type RouteService interface {
	Create(ctx context.Context, route *domain.Route) (uuid.UUID, error)
	Get(ctx context.Context, applicationId, id uuid.UUID) (*domain.Route, error)
	Update(ctx context.Context, route *domain.Route) error
	Delete(ctx context.Context, applicationId, id uuid.UUID) error
	List(ctx context.Context, filter RouteListFilter) ([]domain.Route, error)
	Validate(ctx context.Context, route *domain.Route) []string
}

type RouteListFilter struct {
	ApplicationID  uuid.UUID
	EnvironmentID  string
	IncludeDeleted bool
	Name           string
}

type routeService struct {
	services appservice.ServiceService
	store    repository.Store
}

func NewRouteService(services appservice.ServiceService) RouteService {
	return &routeService{services: services, store: repository.NewPostgresStore()}
}

var DefaultRouteService RouteService = NewRouteService(appservice.DefaultServiceService)

func (s *routeService) Create(ctx context.Context, item *domain.Route) (uuid.UUID, error) {
	if err := s.validate(ctx, item); err != nil {
		return uuid.Nil, err
	}
	return s.store.Create(ctx, item)
}

func (s *routeService) Get(ctx context.Context, applicationId, id uuid.UUID) (*domain.Route, error) {
	return s.store.Get(ctx, applicationId, id)
}

func (s *routeService) Update(ctx context.Context, item *domain.Route) error {
	if err := s.validate(ctx, item); err != nil {
		return err
	}
	return s.store.Update(ctx, item)
}

func (s *routeService) Delete(ctx context.Context, applicationId, id uuid.UUID) error {
	return s.store.Delete(ctx, applicationId, id)
}

func (s *routeService) List(ctx context.Context, filter RouteListFilter) ([]domain.Route, error) {
	return s.store.List(ctx, repository.ListFilter{
		ApplicationID:  filter.ApplicationID,
		EnvironmentID:  filter.EnvironmentID,
		IncludeDeleted: filter.IncludeDeleted,
		Name:           filter.Name,
	})
}

func (s *routeService) Validate(ctx context.Context, item *domain.Route) []string {
	var errs []string
	if item == nil {
		return []string{"route is required"}
	}
	if item.ApplicationID == uuid.Nil {
		errs = append(errs, "application_id is required")
	}
	if strings.TrimSpace(item.EnvironmentID) == "" {
		errs = append(errs, "environment_id is required")
	}
	if strings.TrimSpace(item.Name) == "" {
		errs = append(errs, "name is required")
	}
	if strings.TrimSpace(item.Host) == "" {
		errs = append(errs, "host is required")
	}
	if strings.TrimSpace(item.Path) == "" {
		errs = append(errs, "path is required")
	}
	if strings.TrimSpace(item.ServiceName) == "" {
		errs = append(errs, "service_name is required")
	}
	if item.ServicePort <= 0 {
		errs = append(errs, "service_port is required")
	}
	if len(errs) > 0 {
		return errs
	}
	services, err := s.services.List(ctx, appservice.ServiceListFilter{
		ApplicationID: item.ApplicationID,
		Name:          item.ServiceName,
	})
	if err != nil {
		return []string{err.Error()}
	}
	if len(services) == 0 {
		return []string{"service_name does not exist"}
	}
	for _, port := range services[0].Ports {
		if port.ServicePort == item.ServicePort {
			return nil
		}
	}
	return []string{"service_port does not exist on target service"}
}

func (s *routeService) validate(ctx context.Context, item *domain.Route) error {
	if messages := s.Validate(ctx, item); len(messages) > 0 {
		return sharederrs.JoinInvalid(messages)
	}
	return nil
}
