package domain

import (
	"time"

	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

type Manifest struct {
	model.BaseModel

	ApplicationID          uuid.UUID                `json:"application_id" db:"application_id"`
	EnvironmentID          string                   `json:"-" db:"environment_id"`
	ImageID                uuid.UUID                `json:"image_id" db:"image_id"`
	ImageRef               string                   `json:"image_ref" db:"image_ref"`
	ArtifactRepository     string                   `json:"artifact_repository" db:"artifact_repository"`
	ArtifactTag            string                   `json:"artifact_tag" db:"artifact_tag"`
	ArtifactRef            string                   `json:"artifact_ref" db:"artifact_ref"`
	ArtifactDigest         string                   `json:"artifact_digest" db:"artifact_digest"`
	ArtifactMediaType      string                   `json:"artifact_media_type" db:"artifact_media_type"`
	ArtifactPushedAt       *time.Time               `json:"artifact_pushed_at,omitempty" db:"artifact_pushed_at"`
	ServicesSnapshot       []ManifestService        `json:"services_snapshot" db:"services_snapshot"`
	WorkloadConfigSnapshot ManifestWorkloadConfig   `json:"workload_config_snapshot" db:"workload_config_snapshot"`
	RenderedObjects        []ManifestRenderedObject `json:"rendered_objects" db:"rendered_objects"`
	RenderedYAML           string                   `json:"rendered_yaml" db:"rendered_yaml"`
	Status                 model.ManifestStatus     `json:"status" db:"status"`
}

type CreateManifestRequest struct {
	ApplicationID uuid.UUID `json:"application_id"`
	ImageID       uuid.UUID `json:"image_id"`
}

type ManifestListFilter struct {
	ApplicationID  *uuid.UUID
	ImageID        *uuid.UUID
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

type ManifestRenderedObject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	YAML      string `json:"yaml"`
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
	ManifestID      uuid.UUID                  `json:"manifest_id"`
	ApplicationID   uuid.UUID                  `json:"application_id"`
	Resources       ManifestGroupedResources   `json:"resources"`
	RenderedObjects []ManifestRenderedResource `json:"rendered_objects,omitempty"`
}

func (m *Manifest) CollectionName() string { return "manifests" }
