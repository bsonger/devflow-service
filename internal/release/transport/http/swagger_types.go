package http

import "github.com/google/uuid"

type ReleaseStepDoc struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Progress  int32  `json:"progress"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
}

type IntentDoc struct {
	ID             uuid.UUID `json:"id"`
	Kind           string    `json:"kind"`
	Status         string    `json:"status"`
	ResourceType   string    `json:"resource_type"`
	ResourceID     uuid.UUID `json:"resource_id"`
	TraceID        string    `json:"trace_id,omitempty"`
	Message        string    `json:"message,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	ClaimedBy      string    `json:"claimed_by,omitempty"`
	ClaimedAt      string    `json:"claimed_at,omitempty"`
	LeaseExpiresAt string    `json:"lease_expires_at,omitempty"`
	AttemptCount   int       `json:"attempt_count"`
	CreatedAt      string    `json:"created_at,omitempty"`
	UpdatedAt      string    `json:"updated_at,omitempty"`
}

type ReleaseDoc struct {
	ID                    uuid.UUID           `json:"id"`
	ExecutionIntentID     *uuid.UUID          `json:"execution_intent_id,omitempty"`
	ApplicationID         uuid.UUID           `json:"application_id"`
	ManifestID            uuid.UUID           `json:"manifest_id"`
	EnvironmentID         string              `json:"environment_id"`
	Strategy              string              `json:"strategy"`
	RoutesSnapshot        []ReleaseRouteDoc   `json:"routes_snapshot,omitempty"`
	AppConfigSnapshot     ReleaseAppConfigDoc `json:"app_config_snapshot"`
	ArtifactRepository    string              `json:"artifact_repository,omitempty"`
	ArtifactTag           string              `json:"artifact_tag,omitempty"`
	ArtifactDigest        string              `json:"artifact_digest,omitempty"`
	ArtifactRef           string              `json:"artifact_ref,omitempty"`
	Type                  string              `json:"type"`
	Steps                 []ReleaseStepDoc    `json:"steps,omitempty"`
	Status                string              `json:"status"`
	ArgoCDApplicationName string              `json:"argocd_application_name,omitempty"`
	ExternalRef           string              `json:"external_ref,omitempty"`
	CreatedAt             string              `json:"created_at,omitempty"`
	UpdatedAt             string              `json:"updated_at,omitempty"`
}

type ReleaseRouteDoc struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Path        string `json:"path"`
	ServiceName string `json:"service_name"`
	ServicePort int    `json:"service_port"`
}

type ReleaseFileDoc struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type ReleaseAppConfigDoc struct {
	ID           string            `json:"id,omitempty"`
	Name         string            `json:"name,omitempty"`
	MountPath    string            `json:"mount_path,omitempty"`
	Files        []ReleaseFileDoc  `json:"files,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
	SourcePath   string            `json:"source_path,omitempty"`
	RevisionID   string            `json:"revision_id,omitempty"`
	SourceCommit string            `json:"source_commit,omitempty"`
}

type ReleaseRenderedResourceDoc struct {
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	YAML      string         `json:"yaml"`
	Object    map[string]any `json:"object,omitempty"`
}

type ReleaseBundleFileDoc struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type ReleaseBundleResourcesDoc struct {
	ConfigMap      *ReleaseRenderedResourceDoc  `json:"configmap,omitempty"`
	Deployment     *ReleaseRenderedResourceDoc  `json:"deployment,omitempty"`
	Rollout        *ReleaseRenderedResourceDoc  `json:"rollout,omitempty"`
	Services       []ReleaseRenderedResourceDoc `json:"services,omitempty"`
	VirtualService *ReleaseRenderedResourceDoc  `json:"virtualservice,omitempty"`
}

type ReleaseBundleDoc struct {
	ReleaseID       uuid.UUID                    `json:"release_id"`
	ApplicationID   uuid.UUID                    `json:"application_id"`
	EnvironmentID   string                       `json:"environment_id"`
	Namespace       string                       `json:"namespace,omitempty"`
	ArtifactName    string                       `json:"artifact_name,omitempty"`
	Resources       ReleaseBundleResourcesDoc    `json:"resources"`
	RenderedObjects []ReleaseRenderedResourceDoc `json:"rendered_objects,omitempty"`
	Files           []ReleaseBundleFileDoc       `json:"files,omitempty"`
}

type ManifestStepDoc struct {
	TaskName  string `json:"task_name"`
	TaskRun   string `json:"task_run,omitempty"`
	Status    string `json:"status"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
	Message   string `json:"message,omitempty"`
}

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
	ManifestID    uuid.UUID                   `json:"manifest_id"`
	ApplicationID uuid.UUID                   `json:"application_id"`
	Resources     ManifestGroupedResourcesDoc `json:"resources"`
}

type ManifestDoc struct {
	ID                     uuid.UUID                 `json:"id"`
	ApplicationID          uuid.UUID                 `json:"application_id"`
	GitRevision            string                    `json:"git_revision,omitempty"`
	RepoAddress            string                    `json:"repo_address,omitempty"`
	CommitHash             string                    `json:"commit_hash,omitempty"`
	ImageRef               string                    `json:"image_ref,omitempty"`
	ImageTag               string                    `json:"image_tag,omitempty"`
	ImageDigest            string                    `json:"image_digest,omitempty"`
	PipelineID             string                    `json:"pipeline_id,omitempty"`
	TraceID                string                    `json:"trace_id,omitempty"`
	SpanID                 string                    `json:"span_id,omitempty"`
	Steps                  []ManifestStepDoc         `json:"steps,omitempty"`
	ServicesSnapshot       []ManifestServiceDoc      `json:"services_snapshot"`
	WorkloadConfigSnapshot ManifestWorkloadConfigDoc `json:"workload_config_snapshot"`
	Status                 string                    `json:"status"`
	CreatedAt              string                    `json:"created_at,omitempty"`
	UpdatedAt              string                    `json:"updated_at,omitempty"`
}

type ArgoEventRequest struct {
	ReleaseID   string `json:"release_id" binding:"required"`
	Status      string `json:"status" binding:"required"`
	IntentID    string `json:"intent_id,omitempty"`
	ExternalRef string `json:"external_ref,omitempty"`
	Message     string `json:"message,omitempty"`
}

type ReleaseStepRequest struct {
	ReleaseID string `json:"release_id" binding:"required"`
	StepCode  string `json:"step_code,omitempty"`
	StepName  string `json:"step_name,omitempty"`
	Status    string `json:"status" binding:"required"`
	Progress  int32  `json:"progress,omitempty"`
	Message   string `json:"message,omitempty"`
}

type ReleaseArtifactRequest struct {
	ReleaseID          string `json:"release_id" binding:"required"`
	ArtifactRepository string `json:"artifact_repository,omitempty"`
	ArtifactTag        string `json:"artifact_tag,omitempty"`
	ArtifactDigest     string `json:"artifact_digest,omitempty"`
	ArtifactRef        string `json:"artifact_ref,omitempty"`
	Status             string `json:"status,omitempty"`
	Progress           int32  `json:"progress,omitempty"`
	Message            string `json:"message,omitempty"`
}

type CreateManifestRequestDoc struct {
	ApplicationID string `json:"application_id"`
	GitRevision   string `json:"git_revision,omitempty"`
}
