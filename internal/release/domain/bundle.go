package domain

import "github.com/google/uuid"

type ReleaseBundle struct {
	ReleaseID       uuid.UUID                 `json:"release_id"`
	ApplicationID   uuid.UUID                 `json:"application_id"`
	EnvironmentID   string                    `json:"environment_id"`
	Namespace       string                    `json:"namespace,omitempty"`
	ArtifactName    string                    `json:"artifact_name,omitempty"`
	Resources       ReleaseBundleResources    `json:"resources"`
	RenderedObjects []ReleaseRenderedResource `json:"rendered_objects,omitempty"`
	Files           []ReleaseBundleFile       `json:"files,omitempty"`
}

type ReleaseBundleResources struct {
	ConfigMap      *ReleaseRenderedResource  `json:"configmap,omitempty"`
	Deployment     *ReleaseRenderedResource  `json:"deployment,omitempty"`
	Rollout        *ReleaseRenderedResource  `json:"rollout,omitempty"`
	Services       []ReleaseRenderedResource `json:"services,omitempty"`
	VirtualService *ReleaseRenderedResource  `json:"virtualservice,omitempty"`
}

type ReleaseRenderedResource struct {
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	YAML      string         `json:"yaml"`
	Object    map[string]any `json:"object,omitempty"`
}

type ReleaseBundleFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
