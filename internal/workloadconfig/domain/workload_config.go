package domain

import "github.com/google/uuid"

type WorkloadConfig struct {
	BaseModel

	ApplicationID uuid.UUID      `json:"application_id" db:"application_id"`
	EnvironmentID string         `json:"environment_id,omitempty" db:"environment_id"`
	Name          string         `json:"name" db:"name"`
	Description   string         `json:"description,omitempty" db:"description"`
	Replicas         int            `json:"replicas" db:"replicas"`
	ServiceAccountName string         `json:"service_account_name,omitempty" db:"service_account_name"`
	Resources        map[string]any `json:"resources,omitempty" db:"resources"`
	Probes        map[string]any `json:"probes,omitempty" db:"probes"`
	Env           []EnvVar       `json:"env,omitempty" db:"env"`
	Labels        []LabelItem    `json:"labels,omitempty" db:"labels"`
	WorkloadType  string         `json:"workload_type,omitempty" db:"workload_type"`
	Strategy      string         `json:"strategy,omitempty" db:"strategy"`
}

type WorkloadConfigInput struct {
	ApplicationID uuid.UUID      `json:"application_id"`
	EnvironmentID string         `json:"environment_id,omitempty"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Replicas         int            `json:"replicas"`
	ServiceAccountName string         `json:"service_account_name,omitempty"`
	Resources        map[string]any `json:"resources,omitempty"`
	Probes        map[string]any `json:"probes,omitempty"`
	Env           []EnvVar       `json:"env,omitempty"`
	Labels        []LabelItem    `json:"labels,omitempty"`
	WorkloadType  string         `json:"workload_type,omitempty"`
	Strategy      string         `json:"strategy,omitempty"`
}
