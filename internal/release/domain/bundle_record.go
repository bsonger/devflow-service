package domain

import "github.com/google/uuid"

type ReleaseBundleRecord struct {
	BaseModel

	ReleaseID       uuid.UUID                 `json:"release_id" db:"release_id"`
	Namespace       string                    `json:"namespace" db:"namespace"`
	ArtifactName    string                    `json:"artifact_name" db:"artifact_name"`
	BundleDigest    string                    `json:"bundle_digest" db:"bundle_digest"`
	RenderedObjects []ReleaseRenderedResource `json:"rendered_objects,omitempty" db:"rendered_objects"`
	BundleYAML      string                    `json:"bundle_yaml" db:"bundle_yaml"`
}

func (r *ReleaseBundleRecord) CollectionName() string { return "release_bundles" }
