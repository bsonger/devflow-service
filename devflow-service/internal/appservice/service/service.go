package service

import (
	"context"
	"strings"

	"github.com/bsonger/devflow-service/internal/appservice/domain"
	"github.com/bsonger/devflow-service/internal/appservice/repository"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
)

type ServiceService interface {
	Create(ctx context.Context, service *domain.Service) (uuid.UUID, error)
	Get(ctx context.Context, applicationID, id uuid.UUID) (*domain.Service, error)
	Update(ctx context.Context, service *domain.Service) error
	Delete(ctx context.Context, applicationID, id uuid.UUID) error
	List(ctx context.Context, filter ServiceListFilter) ([]domain.Service, error)
}

type ServiceListFilter struct {
	ApplicationID  uuid.UUID
	IncludeDeleted bool
	Name           string
}

type serviceService struct {
	networks repository.Store
}

func NewServiceService(networks repository.Store) ServiceService {
	return &serviceService{networks: networks}
}

var DefaultServiceService ServiceService = NewServiceService(repository.NetworkStore)

func (s *serviceService) Create(ctx context.Context, item *domain.Service) (uuid.UUID, error) {
	if err := validateService(item); err != nil {
		return uuid.Nil, err
	}
	return s.networks.Create(ctx, &domain.Network{
		BaseModel:     item.BaseModel,
		ApplicationID: item.ApplicationID,
		Name:          item.Name,
		Ports:         toNetworkPorts(item.Ports),
	})
}

func (s *serviceService) Get(ctx context.Context, applicationID, id uuid.UUID) (*domain.Service, error) {
	item, err := s.networks.Get(ctx, applicationID, id)
	if err != nil {
		return nil, err
	}
	return fromNetwork(item), nil
}

func (s *serviceService) Update(ctx context.Context, item *domain.Service) error {
	if err := validateService(item); err != nil {
		return err
	}
	return s.networks.Update(ctx, &domain.Network{
		BaseModel:     item.BaseModel,
		ApplicationID: item.ApplicationID,
		Name:          item.Name,
		Ports:         toNetworkPorts(item.Ports),
	})
}

func (s *serviceService) Delete(ctx context.Context, applicationID, id uuid.UUID) error {
	return s.networks.Delete(ctx, applicationID, id)
}

func (s *serviceService) List(ctx context.Context, filter ServiceListFilter) ([]domain.Service, error) {
	items, err := s.networks.List(ctx, repository.NetworkListFilter{
		ApplicationID:  filter.ApplicationID,
		IncludeDeleted: filter.IncludeDeleted,
		Name:           filter.Name,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Service, 0, len(items))
	for _, item := range items {
		current := item
		out = append(out, *fromNetwork(&current))
	}
	return out, nil
}

func validateService(item *domain.Service) error {
	if item == nil {
		return sharederrs.Required("service")
	}
	if item.ApplicationID == uuid.Nil {
		return sharederrs.Required("application_id")
	}
	if strings.TrimSpace(item.Name) == "" {
		return sharederrs.Required("name")
	}
	if len(item.Ports) == 0 {
		return sharederrs.InvalidArgument("at least one port is required")
	}
	return nil
}

func fromNetwork(item *domain.Network) *domain.Service {
	if item == nil {
		return nil
	}
	return &domain.Service{
		BaseModel:     item.BaseModel,
		ApplicationID: item.ApplicationID,
		Name:          item.Name,
		Ports:         fromNetworkPorts(item.Ports),
	}
}

func toNetworkPorts(ports []domain.ServicePort) []domain.NetworkPort {
	out := make([]domain.NetworkPort, 0, len(ports))
	for _, port := range ports {
		out = append(out, domain.NetworkPort{
			Name:        port.Name,
			ServicePort: port.ServicePort,
			TargetPort:  port.TargetPort,
			Protocol:    port.Protocol,
		})
	}
	return out
}

func fromNetworkPorts(ports []domain.NetworkPort) []domain.ServicePort {
	out := make([]domain.ServicePort, 0, len(ports))
	for _, port := range ports {
		out = append(out, domain.ServicePort{
			Name:        port.Name,
			ServicePort: port.ServicePort,
			TargetPort:  port.TargetPort,
			Protocol:    port.Protocol,
		})
	}
	return out
}
