package domain

import (
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

type Manifest struct {
	model.BaseModel

	ApplicationID          uuid.UUID              `json:"application_id" db:"application_id"`
	GitRevision            string                 `json:"git_revision,omitempty" db:"git_revision"`
	RepoAddress            string                 `json:"repo_address" db:"repo_address"`
	CommitHash             string                 `json:"commit_hash" db:"commit_hash"`
	ImageRef               string                 `json:"image_ref" db:"image_ref"`
	ImageTag               string                 `json:"image_tag,omitempty" db:"image_tag"`
	ImageDigest            string                 `json:"image_digest,omitempty" db:"image_digest"`
	PipelineID             string                 `json:"pipeline_id,omitempty" db:"pipeline_id"`
	TraceID                string                 `json:"trace_id,omitempty" db:"trace_id"`
	SpanID                 string                 `json:"span_id,omitempty" db:"span_id"`
	Steps                  []model.ImageTask      `json:"steps,omitempty" db:"steps"`
	ServicesSnapshot       []ManifestService      `json:"services_snapshot" db:"services_snapshot"`
	WorkloadConfigSnapshot ManifestWorkloadConfig `json:"workload_config_snapshot" db:"workload_config_snapshot"`
	Status                 model.ManifestStatus   `json:"status" db:"status"`
}

type CreateManifestRequest struct {
	ApplicationID uuid.UUID `json:"application_id"`
	GitRevision   string    `json:"git_revision,omitempty"`
}

type ManifestListFilter struct {
	ApplicationID  *uuid.UUID
	IncludeDeleted bool
}

type ManifestServicePort struct {
	Name        string `json:"name,omitempty"`
	ServicePort int    `json:"service_port"`
	TargetPort  int    `json:"target_port"`
	Protocol    string `json:"protocol,omitempty"`
}

type ManifestService struct {
	ID    string                `json:"id,omitempty"`
	Name  string                `json:"name"`
	Ports []ManifestServicePort `json:"ports,omitempty"`
}

type ManifestRoute struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Path        string `json:"path"`
	ServiceName string `json:"service_name"`
	ServicePort int    `json:"service_port"`
}

type ManifestFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type ManifestAppConfig struct {
	ID           string            `json:"id,omitempty"`
	Name         string            `json:"name,omitempty"`
	MountPath    string            `json:"mount_path,omitempty"`
	Files        []ManifestFile    `json:"files,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
	SourcePath   string            `json:"source_path,omitempty"`
	RevisionID   string            `json:"revision_id,omitempty"`
	SourceCommit string            `json:"source_commit,omitempty"`
}

type ManifestWorkloadConfig struct {
	ID           string         `json:"id,omitempty"`
	Name         string         `json:"name,omitempty"`
	Replicas     int            `json:"replicas"`
	Resources    map[string]any `json:"resources,omitempty"`
	Probes       map[string]any `json:"probes,omitempty"`
	Env          []model.EnvVar `json:"env,omitempty"`
	WorkloadType string         `json:"workload_type,omitempty"`
	Strategy     string         `json:"strategy,omitempty"`
}

type ManifestRenderedResource struct {
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	YAML      string         `json:"yaml"`
	Object    map[string]any `json:"object,omitempty"`
}

type ManifestGroupedResources struct {
	ConfigMap      *ManifestRenderedResource  `json:"configmap,omitempty"`
	Deployment     *ManifestRenderedResource  `json:"deployment,omitempty"`
	Rollout        *ManifestRenderedResource  `json:"rollout,omitempty"`
	Services       []ManifestRenderedResource `json:"services,omitempty"`
	VirtualService *ManifestRenderedResource  `json:"virtualservice,omitempty"`
}

type ManifestResourcesView struct {
	ManifestID    uuid.UUID                `json:"manifest_id"`
	ApplicationID uuid.UUID                `json:"application_id"`
	Resources     ManifestGroupedResources `json:"resources"`
}

func (m *Manifest) CollectionName() string { return "manifests" }
