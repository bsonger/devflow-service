package domain

import "github.com/google/uuid"

type RenderedConfigMap struct {
	Data map[string]string `json:"data,omitempty"`
}

type AppConfig struct {
	BaseModel

	ApplicationID     uuid.UUID         `json:"application_id" db:"application_id"`
	EnvironmentID     string            `json:"environment_id" db:"env"`
	Name              string            `json:"name" db:"name"`
	Description       string            `json:"description,omitempty" db:"description"`
	Format            string            `json:"format,omitempty" db:"format"`
	Data              string            `json:"data,omitempty" db:"data"`
	MountPath         string            `json:"mount_path,omitempty" db:"mount_path"`
	Labels            []LabelItem       `json:"labels,omitempty" db:"labels"`
	SourcePath        string            `json:"source_path" db:"source_path"`
	LatestRevisionNo  int               `json:"latest_revision_no" db:"latest_revision_no"`
	LatestRevisionID  *uuid.UUID        `json:"latest_revision_id,omitempty" db:"latest_revision_id"`
	Files             []File            `json:"files,omitempty"`
	RenderedConfigMap RenderedConfigMap `json:"rendered_configmap,omitempty"`
	SourceCommit      string            `json:"source_commit,omitempty"`
}

type AppConfigRevision struct {
	ID                uuid.UUID         `json:"id" db:"id"`
	AppConfigID       uuid.UUID         `json:"app_config_id" db:"configuration_id"`
	RevisionNo        int               `json:"revision_no" db:"revision_no"`
	Files             []File            `json:"files" db:"files"`
	RenderedConfigMap RenderedConfigMap `json:"rendered_configmap" db:"rendered_configmap"`
	ContentHash       string            `json:"content_hash" db:"content_hash"`
	SourceCommit      string            `json:"source_commit" db:"source_commit"`
	SourceDigest      string            `json:"source_digest,omitempty" db:"source_digest"`
	CreatedAt         string            `json:"created_at,omitempty" db:"created_at"`
}

type AppConfigInput struct {
	ApplicationID uuid.UUID   `json:"application_id"`
	EnvironmentID string      `json:"environment_id"`
	Name          string      `json:"name"`
	Description   string      `json:"description,omitempty"`
	Format        string      `json:"format,omitempty"`
	Data          string      `json:"data,omitempty"`
	MountPath     string      `json:"mount_path,omitempty"`
	Labels        []LabelItem `json:"labels,omitempty"`
	SourcePath    string      `json:"source_path,omitempty"`
}
