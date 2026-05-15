package domain

import "github.com/google/uuid"

// WorkloadSizeClass is the constrained user-facing size selector for workload resources.
// Later slices validate and expand this enum into concrete Kubernetes CPU/memory requests and limits.
type WorkloadSizeClass string

const (
	WorkloadSizeClassSmall  WorkloadSizeClass = "small"
	WorkloadSizeClassMedium WorkloadSizeClass = "medium"
	WorkloadSizeClassLarge  WorkloadSizeClass = "large"
	WorkloadSizeClassXLarge WorkloadSizeClass = "xlarge"
)

// WorkloadSizeClassResources is the fixed mapping table that downstream validation, migration,
// and render-time expansion must share instead of inventing parallel resource defaults.
var WorkloadSizeClassResources = map[WorkloadSizeClass]WorkloadResourceRequirements{
	WorkloadSizeClassSmall: {
		SizeClass: WorkloadSizeClassSmall,
		Requests:  WorkloadResourceList{CPU: "100m", Memory: "128Mi"},
		Limits:    WorkloadResourceList{CPU: "500m", Memory: "512Mi"},
	},
	WorkloadSizeClassMedium: {
		SizeClass: WorkloadSizeClassMedium,
		Requests:  WorkloadResourceList{CPU: "250m", Memory: "256Mi"},
		Limits:    WorkloadResourceList{CPU: "1", Memory: "1Gi"},
	},
	WorkloadSizeClassLarge: {
		SizeClass: WorkloadSizeClassLarge,
		Requests:  WorkloadResourceList{CPU: "500m", Memory: "512Mi"},
		Limits:    WorkloadResourceList{CPU: "2", Memory: "2Gi"},
	},
	WorkloadSizeClassXLarge: {
		SizeClass: WorkloadSizeClassXLarge,
		Requests:  WorkloadResourceList{CPU: "1", Memory: "1Gi"},
		Limits:    WorkloadResourceList{CPU: "4", Memory: "4Gi"},
	},
}

// WorkloadResourceList mirrors the CPU/memory keys a later renderer will project into container resources.
type WorkloadResourceList struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// WorkloadResourceRequirements is the constrained API-facing replacement for the legacy free-form resources map.
type WorkloadResourceRequirements struct {
	SizeClass WorkloadSizeClass    `json:"size_class,omitempty"`
	Requests  WorkloadResourceList `json:"requests,omitempty"`
	Limits    WorkloadResourceList `json:"limits,omitempty"`
}

// WorkloadProbe models one HTTP probe contract row. Later slices are responsible for validation and render-time translation.
type WorkloadProbe struct {
	Path                string `json:"path,omitempty"`
	Port                string `json:"port,omitempty"`
	InitialDelaySeconds int    `json:"initial_delay_seconds,omitempty"`
	PeriodSeconds       int    `json:"period_seconds,omitempty"`
	TimeoutSeconds      int    `json:"timeout_seconds,omitempty"`
	FailureThreshold    int    `json:"failure_threshold,omitempty"`
}

// WorkloadProbes is the constrained replacement for the legacy wide probes map.
type WorkloadProbes struct {
	Liveness  *WorkloadProbe `json:"liveness,omitempty"`
	Readiness *WorkloadProbe `json:"readiness,omitempty"`
	Startup   *WorkloadProbe `json:"startup,omitempty"`
}

type WorkloadMetricsScrapeProfile string

const (
	WorkloadMetricsScrapeProfileDefault WorkloadMetricsScrapeProfile = "default"
	WorkloadMetricsScrapeProfileFast    WorkloadMetricsScrapeProfile = "fast"
	WorkloadMetricsScrapeProfileSlow    WorkloadMetricsScrapeProfile = "slow"
)

type WorkloadMetrics struct {
	Enabled       bool                         `json:"enabled,omitempty"`
	Port          int                          `json:"port,omitempty"`
	ScrapeProfile WorkloadMetricsScrapeProfile `json:"scrape_profile,omitempty"`
}

type WorkloadEmptyDir struct {
	Name      string `json:"name,omitempty"`
	MountPath string `json:"mount_path,omitempty"`
	Medium    string `json:"medium,omitempty"`
	SizeLimit string `json:"size_limit,omitempty"`
}

// WorkloadConfig stores the application-scoped runtime workload contract used by config-service handlers.
type WorkloadConfig struct {
	BaseModel

	ApplicationID      uuid.UUID                    `json:"application_id" db:"application_id"`
	Replicas           int                          `json:"replicas" db:"replicas"`
	ServiceAccountName string                       `json:"service_account_name,omitempty" db:"service_account_name"`
	Resources          WorkloadResourceRequirements `json:"resources,omitempty" db:"resources"`
	Probes             WorkloadProbes               `json:"probes,omitempty" db:"probes"`
	Metrics            WorkloadMetrics              `json:"metrics,omitempty" db:"metrics"`
	EmptyDirs          []WorkloadEmptyDir           `json:"empty_dirs,omitempty" db:"empty_dirs"`
	Env                []EnvVar                     `json:"env,omitempty" db:"env"`
	Labels             map[string]string            `json:"labels,omitempty" db:"labels"`
	Annotations        map[string]string            `json:"annotations,omitempty" db:"annotations"`
}

// WorkloadConfigInput is the public create/update payload shared by the HTTP transport and generated docs.
type WorkloadConfigInput struct {
	ApplicationID      uuid.UUID                    `json:"application_id"`
	Replicas           int                          `json:"replicas"`
	ServiceAccountName string                       `json:"service_account_name,omitempty"`
	Resources          WorkloadResourceRequirements `json:"resources,omitempty"`
	Probes             WorkloadProbes               `json:"probes,omitempty"`
	Metrics            WorkloadMetrics              `json:"metrics,omitempty"`
	EmptyDirs          []WorkloadEmptyDir           `json:"empty_dirs,omitempty"`
	Env                []EnvVar                     `json:"env,omitempty"`
	Labels             map[string]string            `json:"labels,omitempty"`
	Annotations        map[string]string            `json:"annotations,omitempty"`
}
