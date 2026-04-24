package domain

import "github.com/google/uuid"

type Route struct {
	BaseModel

	ApplicationID uuid.UUID `json:"application_id" db:"application_id"`
	Name          string    `json:"name" db:"name"`
	Host          string    `json:"host" db:"host"`
	Path          string    `json:"path" db:"path"`
	ServiceName   string    `json:"service_name" db:"service_name"`
	ServicePort   int       `json:"service_port" db:"service_port"`
}

type RouteInput struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Path        string `json:"path"`
	ServiceName string `json:"service_name"`
	ServicePort int    `json:"service_port"`
}

type RouteValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}
