package http

import "github.com/google/uuid"

type ManifestServicePortDoc struct {
	Name        string `json:"name,omitempty"`
	ServicePort int    `json:"service_port"`
	TargetPort  int    `json:"target_port"`
	Protocol    string `json:"protocol,omitempty"`
}

type ManifestServiceDoc struct {
	ID    string                   `json:"id,omitempty"`
	Name  string                   `json:"name"`
	Ports []ManifestServicePortDoc `json:"ports,omitempty"`
}

type ManifestRouteDoc struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Path        string `json:"path"`
	ServiceName string `json:"service_name"`
	ServicePort int    `json:"service_port"`
}

type ManifestFileDoc struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type ManifestAppConfigDoc struct {
	ID           string            `json:"id,omitempty"`
	Name         string            `json:"name,omitempty"`
	Files        []ManifestFileDoc `json:"files,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
	SourcePath   string            `json:"source_path,omitempty"`
	RevisionID   string            `json:"revision_id,omitempty"`
	SourceCommit string            `json:"source_commit,omitempty"`
}

type ManifestEnvVarDoc struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ManifestWorkloadConfigDoc struct {
	ID           string              `json:"id,omitempty"`
	Name         string              `json:"name,omitempty"`
	Replicas     int                 `json:"replicas"`
	Resources    map[string]any      `json:"resources,omitempty"`
	Probes       map[string]any      `json:"probes,omitempty"`
	Env          []ManifestEnvVarDoc `json:"env,omitempty"`
	WorkloadType string              `json:"workload_type,omitempty"`
	Strategy     string              `json:"strategy,omitempty"`
}

type ManifestRenderedObjectDoc struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	YAML      string `json:"yaml"`
}

type ManifestRenderedResourceDoc struct {
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	YAML      string         `json:"yaml"`
	Object    map[string]any `json:"object,omitempty"`
}

type ManifestGroupedResourcesDoc struct {
	ConfigMap      *ManifestRenderedResourceDoc  `json:"configmap,omitempty"`
	Deployment     *ManifestRenderedResourceDoc  `json:"deployment,omitempty"`
	Rollout        *ManifestRenderedResourceDoc  `json:"rollout,omitempty"`
	Services       []ManifestRenderedResourceDoc `json:"services,omitempty"`
	VirtualService *ManifestRenderedResourceDoc  `json:"virtualservice,omitempty"`
}

type ManifestResourcesViewDoc struct {
	ManifestID      uuid.UUID                     `json:"manifest_id"`
	ApplicationID   uuid.UUID                     `json:"application_id"`
	Resources       ManifestGroupedResourcesDoc   `json:"resources"`
	RenderedObjects []ManifestRenderedResourceDoc `json:"rendered_objects,omitempty"`
}

type ManifestDoc struct {
	ID                     uuid.UUID                   `json:"id"`
	ApplicationID          uuid.UUID                   `json:"application_id"`
	ImageID                uuid.UUID                   `json:"image_id"`
	ImageRef               string                      `json:"image_ref"`
	ArtifactRepository     string                      `json:"artifact_repository,omitempty"`
	ArtifactTag            string                      `json:"artifact_tag,omitempty"`
	ArtifactRef            string                      `json:"artifact_ref,omitempty"`
	ArtifactDigest         string                      `json:"artifact_digest,omitempty"`
	ArtifactMediaType      string                      `json:"artifact_media_type,omitempty"`
	ArtifactPushedAt       string                      `json:"artifact_pushed_at,omitempty"`
	ServicesSnapshot       []ManifestServiceDoc        `json:"services_snapshot"`
	WorkloadConfigSnapshot ManifestWorkloadConfigDoc   `json:"workload_config_snapshot"`
	RenderedObjects        []ManifestRenderedObjectDoc `json:"rendered_objects"`
	RenderedYAML           string                      `json:"rendered_yaml"`
	Status                 string                      `json:"status"`
	CreatedAt              string                      `json:"created_at,omitempty"`
	UpdatedAt              string                      `json:"updated_at,omitempty"`
}
