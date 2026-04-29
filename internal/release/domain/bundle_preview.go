package domain

import (
	"time"

	"github.com/google/uuid"
)

type ReleaseBundlePreview struct {
	ReleaseID      uuid.UUID                 `json:"release_id"`
	ManifestID     uuid.UUID                 `json:"manifest_id"`
	ApplicationID  uuid.UUID                 `json:"application_id"`
	EnvironmentID  string                    `json:"environment_id"`
	Strategy       string                    `json:"strategy"`
	Namespace      string                    `json:"namespace"`
	ArtifactName   string                    `json:"artifact_name,omitempty"`
	BundleDigest   string                    `json:"bundle_digest,omitempty"`
	Artifact       *ReleaseBundleArtifact    `json:"artifact,omitempty"`
	FrozenInputs   ReleaseBundleFrozenInputs `json:"frozen_inputs"`
	RenderedBundle ReleaseRenderedBundleView `json:"rendered_bundle"`
	RenderedAt     time.Time                 `json:"rendered_at"`
	PublishedAt    *time.Time                `json:"published_at,omitempty"`
}

type ReleaseBundleSummary struct {
	Available           bool                        `json:"available"`
	Namespace           string                      `json:"namespace,omitempty"`
	ArtifactName        string                      `json:"artifact_name,omitempty"`
	BundleDigest        string                      `json:"bundle_digest,omitempty"`
	PrimaryWorkloadKind string                      `json:"primary_workload_kind,omitempty"`
	ResourceCounts      ReleaseBundleResourceCounts `json:"resource_counts,omitempty"`
	Artifact            *ReleaseBundleArtifact      `json:"artifact,omitempty"`
	RenderedAt          *time.Time                  `json:"rendered_at,omitempty"`
	PublishedAt         *time.Time                  `json:"published_at,omitempty"`
}

type ReleaseBundleResourceCounts struct {
	ConfigMaps      int `json:"configmaps,omitempty"`
	Services        int `json:"services,omitempty"`
	Deployments     int `json:"deployments,omitempty"`
	Rollouts        int `json:"rollouts,omitempty"`
	VirtualServices int `json:"virtualservices,omitempty"`
	Total           int `json:"total,omitempty"`
}

type ReleaseBundleArtifact struct {
	Repository string `json:"repository,omitempty"`
	Tag        string `json:"tag,omitempty"`
	Digest     string `json:"digest,omitempty"`
	Ref        string `json:"ref,omitempty"`
}

type ReleaseBundleFrozenInputs struct {
	ManifestSummary ReleaseBundleManifestSummary `json:"manifest_summary"`
	Services        []ReleaseFrozenService       `json:"services,omitempty"`
	Workload        ReleaseFrozenWorkload        `json:"workload"`
	AppConfig       ReleaseAppConfig             `json:"app_config"`
	Routes          []ReleaseRoute               `json:"routes,omitempty"`
}

type ReleaseBundleManifestSummary struct {
	ManifestID  uuid.UUID `json:"manifest_id"`
	CommitHash  string    `json:"commit_hash,omitempty"`
	ImageRef    string    `json:"image_ref,omitempty"`
	ImageDigest string    `json:"image_digest,omitempty"`
}

type ReleaseFrozenService struct {
	Name  string                     `json:"name"`
	Ports []ReleaseFrozenServicePort `json:"ports,omitempty"`
}

type ReleaseFrozenServicePort struct {
	Name        string `json:"name,omitempty"`
	ServicePort int    `json:"service_port"`
	TargetPort  int    `json:"target_port"`
	Protocol    string `json:"protocol,omitempty"`
}

type ReleaseFrozenWorkload struct {
	Replicas           int               `json:"replicas"`
	ServiceAccountName string            `json:"service_account_name,omitempty"`
	Resources          map[string]any    `json:"resources,omitempty"`
	Probes             map[string]any    `json:"probes,omitempty"`
	Env                []EnvVar          `json:"env,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
	Annotations        map[string]string `json:"annotations,omitempty"`
}

type ReleaseRenderedBundleView struct {
	ResourceGroups    []ReleaseResourceGroup        `json:"resource_groups,omitempty"`
	RenderedResources []ReleaseRenderedResourceView `json:"rendered_resources,omitempty"`
	Files             []ReleaseBundleFileView       `json:"files,omitempty"`
}

type ReleaseResourceGroup struct {
	Kind  string                       `json:"kind"`
	Items []ReleaseRenderedResourceRef `json:"items,omitempty"`
}

type ReleaseRenderedResourceRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type ReleaseRenderedResourceView struct {
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace,omitempty"`
	Summary   map[string]any `json:"summary,omitempty"`
	YAML      string         `json:"yaml,omitempty"`
}

type ReleaseBundleFileView struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
