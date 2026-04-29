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

type RuntimeObservedWorkload struct {
	ID                  uuid.UUID                          `json:"id" db:"id"`
	RuntimeSpecID       uuid.UUID                          `json:"runtime_spec_id" db:"runtime_spec_id"`
	ApplicationID       uuid.UUID                          `json:"application_id" db:"application_id"`
	Environment         string                             `json:"environment" db:"environment"`
	Namespace           string                             `json:"namespace" db:"namespace"`
	WorkloadKind        string                             `json:"workload_kind" db:"workload_kind"`
	WorkloadName        string                             `json:"workload_name" db:"workload_name"`
	DesiredReplicas     int                                `json:"desired_replicas" db:"desired_replicas"`
	ReadyReplicas       int                                `json:"ready_replicas" db:"ready_replicas"`
	UpdatedReplicas     int                                `json:"updated_replicas" db:"updated_replicas"`
	AvailableReplicas   int                                `json:"available_replicas" db:"available_replicas"`
	UnavailableReplicas int                                `json:"unavailable_replicas" db:"unavailable_replicas"`
	ObservedGeneration  int64                              `json:"observed_generation" db:"observed_generation"`
	SummaryStatus       string                             `json:"summary_status" db:"summary_status"`
	Images              []string                           `json:"images,omitempty" db:"images_jsonb"`
	Conditions          []RuntimeObservedWorkloadCondition `json:"conditions,omitempty" db:"conditions_jsonb"`
	Labels              map[string]string                  `json:"labels,omitempty" db:"labels_jsonb"`
	Annotations         map[string]string                  `json:"annotations,omitempty" db:"annotations_jsonb"`
	ObservedAt          time.Time                          `json:"observed_at" db:"observed_at"`
	RestartAt           *time.Time                         `json:"restart_at,omitempty" db:"restart_at"`
	DeletedAt           *time.Time                         `json:"deleted_at,omitempty" db:"deleted_at"`
}

type RuntimeObservedWorkloadCondition struct {
	Type               string     `json:"type"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason,omitempty"`
	Message            string     `json:"message,omitempty"`
	LastTransitionTime *time.Time `json:"last_transition_time,omitempty"`
}

type RuntimeOperation struct {
	ID            uuid.UUID `json:"id" db:"id"`
	RuntimeSpecID uuid.UUID `json:"runtime_spec_id" db:"runtime_spec_id"`
	OperationType string    `json:"operation_type" db:"operation_type"`
	TargetName    string    `json:"target_name" db:"target_name"`
	Operator      string    `json:"operator" db:"operator"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}
