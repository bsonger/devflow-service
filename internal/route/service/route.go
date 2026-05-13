package service

import (
	"context"
	"strings"

	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	"github.com/bsonger/devflow-service/internal/route/domain"
	"github.com/bsonger/devflow-service/internal/route/repository"
	servicesvc "github.com/bsonger/devflow-service/internal/service/service"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
	"go.uber.org/zap"
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
	services servicesvc.ServiceService
	store    repository.Store
}

func NewRouteService(services servicesvc.ServiceService) RouteService {
	return &routeService{services: services, store: repository.NewPostgresStore()}
}

var DefaultRouteService RouteService = NewRouteService(servicesvc.DefaultServiceService)

func (s *routeService) Create(ctx context.Context, item *domain.Route) (uuid.UUID, error) {
	log := platformobs.OperationLogger(ctx, "network_service", "create_route", "route",
		zap.String("devflow.application.id", routeApplicationID(item)),
		zap.String("devflow.environment.id", routeEnvironmentID(item)),
	)
	if err := s.validate(ctx, item); err != nil {
		platformobs.LogOperationFailure(log, "create route failed", err)
		return uuid.Nil, err
	}
	id, err := s.store.Create(ctx, item)
	if err != nil {
		platformobs.LogOperationFailure(log, "create route failed", err)
		return uuid.Nil, err
	}
	platformobs.LogOperationSuccess(log, "route created",
		zap.String("resource_id", id.String()),
	)
	return id, nil
}

func (s *routeService) Get(ctx context.Context, applicationId, id uuid.UUID) (*domain.Route, error) {
	return s.store.Get(ctx, applicationId, id)
}

func (s *routeService) Update(ctx context.Context, item *domain.Route) error {
	log := platformobs.OperationLogger(ctx, "network_service", "update_route", "route",
		zap.String("resource_id", routeID(item)),
		zap.String("devflow.application.id", routeApplicationID(item)),
		zap.String("devflow.environment.id", routeEnvironmentID(item)),
	)
	if err := s.validate(ctx, item); err != nil {
		platformobs.LogOperationFailure(log, "update route failed", err)
		return err
	}
	if err := s.store.Update(ctx, item); err != nil {
		platformobs.LogOperationFailure(log, "update route failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "route updated")
	return nil
}

func (s *routeService) Delete(ctx context.Context, applicationId, id uuid.UUID) error {
	log := platformobs.OperationLogger(ctx, "network_service", "delete_route", "route",
		zap.String("resource_id", id.String()),
		zap.String("devflow.application.id", applicationId.String()),
	)
	if err := s.store.Delete(ctx, applicationId, id); err != nil {
		platformobs.LogOperationFailure(log, "delete route failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "route deleted")
	return nil
}

func (s *routeService) List(ctx context.Context, filter RouteListFilter) ([]domain.Route, error) {
	return s.store.List(ctx, repository.ListFilter{
		ApplicationID:  filter.ApplicationID,
		EnvironmentID:  filter.EnvironmentID,
		IncludeDeleted: filter.IncludeDeleted,
		Name:           filter.Name,
	})
}

func routeID(item *domain.Route) string {
	if item == nil || item.ID == uuid.Nil {
		return ""
	}
	return item.ID.String()
}

func routeApplicationID(item *domain.Route) string {
	if item == nil || item.ApplicationID == uuid.Nil {
		return ""
	}
	return item.ApplicationID.String()
}

func routeEnvironmentID(item *domain.Route) string {
	if item == nil {
		return ""
	}
	return strings.TrimSpace(item.EnvironmentID)
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
	services, err := s.services.List(ctx, servicesvc.ServiceListFilter{
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
