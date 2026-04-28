package http

import "github.com/google/uuid"

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
	ID                 string              `json:"id,omitempty"`
	Replicas           int                 `json:"replicas"`
	ServiceAccountName string              `json:"service_account_name,omitempty"`
	Resources          map[string]any      `json:"resources,omitempty"`
	Probes             map[string]any      `json:"probes,omitempty"`
	Env                []ManifestEnvVarDoc `json:"env,omitempty"`
	Labels             map[string]string   `json:"labels,omitempty"`
	Annotations        map[string]string   `json:"annotations,omitempty"`
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

type CreateManifestRequestDoc struct {
	ApplicationID string `json:"application_id"`
	GitRevision   string `json:"git_revision,omitempty"`
}
