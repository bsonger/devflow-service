package service

import (
	"strings"
	"testing"

	"github.com/bsonger/devflow-service/internal/service/domain"
	"github.com/google/uuid"
)

func TestValidateServiceDefaultsProtocol(t *testing.T) {
	item := &domain.Service{
		ApplicationID: uuid.New(),
		Name:          "demo",
		Ports:         []domain.ServicePort{{ServicePort: 80, TargetPort: 8080}},
	}

	if err := validateService(item); err != nil {
		t.Fatalf("validateService returned error: %v", err)
	}
	if got := item.Ports[0].Protocol; got != "TCP" {
		t.Fatalf("protocol = %q, want TCP", got)
	}
}

func TestValidateServiceRequiresTargetPort(t *testing.T) {
	item := &domain.Service{
		ApplicationID: uuid.New(),
		Name:          "demo",
		Ports:         []domain.ServicePort{{ServicePort: 80}},
	}

	err := validateService(item)
	if err == nil || !strings.Contains(err.Error(), "target_port") {
		t.Fatalf("expected target_port validation error, got %v", err)
	}
}

func TestValidateServiceRequiresPortNamesForMultiPort(t *testing.T) {
	item := &domain.Service{
		ApplicationID: uuid.New(),
		Name:          "demo",
		Ports: []domain.ServicePort{
			{ServicePort: 80, TargetPort: 8080, Name: "http"},
			{ServicePort: 81, TargetPort: 8081},
		},
	}

	err := validateService(item)
	if err == nil || !strings.Contains(err.Error(), "name is required for multi-port services") {
		t.Fatalf("expected multi-port name validation error, got %v", err)
	}
}

func TestValidateServiceRejectsDuplicatePortNames(t *testing.T) {
	item := &domain.Service{
		ApplicationID: uuid.New(),
		Name:          "demo",
		Ports: []domain.ServicePort{
			{ServicePort: 80, TargetPort: 8080, Name: "http"},
			{ServicePort: 81, TargetPort: 8081, Name: "http"},
		},
	}

	err := validateService(item)
	if err == nil || !strings.Contains(err.Error(), "must be unique") {
		t.Fatalf("expected duplicate name validation error, got %v", err)
	}
}
