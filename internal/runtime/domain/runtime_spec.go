package domain

import (
	"time"

	"github.com/google/uuid"
)

type RuntimeSpec struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	ApplicationID     uuid.UUID  `json:"application_id" db:"application_id"`
	Environment       string     `json:"environment" db:"environment"`
	CurrentRevisionID *uuid.UUID `json:"current_revision_id,omitempty" db:"current_revision_id"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

type RuntimeSpecRevision struct {
	ID               uuid.UUID `json:"id" db:"id"`
	RuntimeSpecID    uuid.UUID `json:"runtime_spec_id" db:"runtime_spec_id"`
	Revision         int       `json:"revision" db:"revision"`
	Replicas         int       `json:"replicas" db:"replicas"`
	HealthThresholds string    `json:"health_thresholds" db:"health_thresholds_jsonb"`
	Resources        string    `json:"resources" db:"resources_jsonb"`
	Autoscaling      string    `json:"autoscaling" db:"autoscaling_jsonb"`
	Scheduling       string    `json:"scheduling" db:"scheduling_jsonb"`
	PodEnvs          string    `json:"pod_envs" db:"pod_envs_jsonb"`
	CreatedBy        string    `json:"created_by" db:"created_by"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

type RuntimeObservedPod struct {
	ID            uuid.UUID                     `json:"id" db:"id"`
	RuntimeSpecID uuid.UUID                     `json:"runtime_spec_id" db:"runtime_spec_id"`
	ApplicationID uuid.UUID                     `json:"application_id" db:"application_id"`
	Environment   string                        `json:"environment" db:"environment"`
	Namespace     string                        `json:"namespace" db:"namespace"`
	PodName       string                        `json:"pod_name" db:"pod_name"`
	Phase         string                        `json:"phase" db:"phase"`
	Ready         bool                          `json:"ready" db:"ready"`
	Restarts      int                           `json:"restarts" db:"restarts"`
	NodeName      string                        `json:"node_name" db:"node_name"`
	PodIP         string                        `json:"pod_ip" db:"pod_ip"`
	HostIP        string                        `json:"host_ip" db:"host_ip"`
	OwnerKind     string                        `json:"owner_kind" db:"owner_kind"`
	OwnerName     string                        `json:"owner_name" db:"owner_name"`
	Labels        map[string]string             `json:"labels,omitempty" db:"labels_jsonb"`
	Containers    []RuntimeObservedPodContainer `json:"containers,omitempty" db:"containers_jsonb"`
	ObservedAt    time.Time                     `json:"observed_at" db:"observed_at"`
	DeletedAt     *time.Time                    `json:"deleted_at,omitempty" db:"deleted_at"`
}

type RuntimeObservedPodContainer struct {
	Name         string `json:"name"`
	Image        string `json:"image,omitempty"`
	ImageID      string `json:"image_id,omitempty"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restart_count"`
	State        string `json:"state,omitempty"`
}

type RuntimeOperation struct {
	ID            uuid.UUID `json:"id" db:"id"`
	RuntimeSpecID uuid.UUID `json:"runtime_spec_id" db:"runtime_spec_id"`
	OperationType string    `json:"operation_type" db:"operation_type"`
	TargetName    string    `json:"target_name" db:"target_name"`
	Operator      string    `json:"operator" db:"operator"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}
