package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/bsonger/devflow-service/internal/service/domain"
	"github.com/bsonger/devflow-service/internal/service/repository"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
)

type ServiceService interface {
	Create(ctx context.Context, service *domain.Service) (uuid.UUID, error)
	Get(ctx context.Context, applicationId, id uuid.UUID) (*domain.Service, error)
	Update(ctx context.Context, service *domain.Service) error
	Delete(ctx context.Context, applicationId, id uuid.UUID) error
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

func (s *serviceService) Get(ctx context.Context, applicationId, id uuid.UUID) (*domain.Service, error) {
	item, err := s.networks.Get(ctx, applicationId, id)
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

func (s *serviceService) Delete(ctx context.Context, applicationId, id uuid.UUID) error {
	return s.networks.Delete(ctx, applicationId, id)
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

	seenPortNames := make(map[string]struct{}, len(item.Ports))
	multiPort := len(item.Ports) > 1
	for i := range item.Ports {
		port := &item.Ports[i]
		if port.ServicePort <= 0 {
			return sharederrs.InvalidArgument(fmt.Sprintf("ports[%d].service_port is required", i))
		}
		if port.TargetPort <= 0 {
			return sharederrs.InvalidArgument(fmt.Sprintf("ports[%d].target_port is required", i))
		}

		portName := strings.TrimSpace(port.Name)
		if multiPort && portName == "" {
			return sharederrs.InvalidArgument(fmt.Sprintf("ports[%d].name is required for multi-port services", i))
		}
		if portName != "" {
			if _, exists := seenPortNames[portName]; exists {
				return sharederrs.InvalidArgument(fmt.Sprintf("ports[%d].name must be unique", i))
			}
			seenPortNames[portName] = struct{}{}
			port.Name = portName
		}

		protocol := strings.ToUpper(strings.TrimSpace(port.Protocol))
		if protocol == "" {
			protocol = "TCP"
		}
		switch protocol {
		case "TCP", "UDP", "SCTP":
			port.Protocol = protocol
		default:
			return sharederrs.InvalidArgument(fmt.Sprintf("ports[%d].protocol must be one of TCP, UDP, SCTP", i))
		}
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
