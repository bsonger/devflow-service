package domain

import "github.com/google/uuid"

type ServicePort struct {
	Name        string `json:"name,omitempty"`
	ServicePort int    `json:"service_port"`
	TargetPort  int    `json:"target_port"`
	Protocol    string `json:"protocol,omitempty"`
}

type Service struct {
	BaseModel

	ApplicationID uuid.UUID     `json:"application_id" db:"application_id"`
	Name          string        `json:"name" db:"name"`
	Ports         []ServicePort `json:"ports,omitempty" db:"ports"`
}

type ServiceInput struct {
	ApplicationID uuid.UUID     `json:"application_id"`
	Name          string        `json:"name"`
	Ports         []ServicePort `json:"ports,omitempty"`
}
