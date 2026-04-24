package domain

import "github.com/google/uuid"

type NetworkPort struct {
	Name        string `json:"name,omitempty"`
	ServicePort int    `json:"service_port"`
	TargetPort  int    `json:"target_port"`
	Protocol    string `json:"protocol,omitempty"`
}

type Network struct {
	BaseModel

	ApplicationID uuid.UUID     `json:"application_id" db:"application_id"`
	Name          string        `json:"name" db:"name"`
	Ports         []NetworkPort `json:"ports,omitempty" db:"ports"`
	Hosts         []string      `json:"hosts,omitempty" db:"hosts"`
	Paths         []string      `json:"paths,omitempty" db:"paths"`
	GatewayRefs   []string      `json:"gateway_refs,omitempty" db:"gateway_refs"`
	Visibility    string        `json:"visibility,omitempty" db:"visibility"`
}

type NetworkInput struct {
	Name        string        `json:"name"`
	Ports       []NetworkPort `json:"ports,omitempty"`
	Hosts       []string      `json:"hosts,omitempty"`
	Paths       []string      `json:"paths,omitempty"`
	GatewayRefs []string      `json:"gateway_refs,omitempty"`
	Visibility  string        `json:"visibility,omitempty"`
}
