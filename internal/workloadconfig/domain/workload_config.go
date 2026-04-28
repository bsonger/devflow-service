package domain

import "github.com/google/uuid"

type WorkloadConfig struct {
	BaseModel

	ApplicationID      uuid.UUID         `json:"application_id" db:"application_id"`
	Replicas           int               `json:"replicas" db:"replicas"`
	ServiceAccountName string            `json:"service_account_name,omitempty" db:"service_account_name"`
	Resources          map[string]any    `json:"resources,omitempty" db:"resources"`
	Probes             map[string]any    `json:"probes,omitempty" db:"probes"`
	Env                []EnvVar          `json:"env,omitempty" db:"env"`
	Labels             map[string]string `json:"labels,omitempty" db:"labels"`
	Annotations        map[string]string `json:"annotations,omitempty" db:"annotations"`
}

type WorkloadConfigInput struct {
	ApplicationID      uuid.UUID         `json:"application_id"`
	Replicas           int               `json:"replicas"`
	ServiceAccountName string            `json:"service_account_name,omitempty"`
	Resources          map[string]any    `json:"resources,omitempty"`
	Probes             map[string]any    `json:"probes,omitempty"`
	Env                []EnvVar          `json:"env,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
	Annotations        map[string]string `json:"annotations,omitempty"`
}
