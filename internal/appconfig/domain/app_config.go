package domain

import "github.com/google/uuid"

type AppConfig struct {
	BaseModel

	ApplicationID    uuid.UUID  `json:"application_id" db:"application_id"`
	EnvironmentID    string     `json:"environment_id" db:"env"`
	MountPath        string     `json:"mount_path,omitempty" db:"mount_path"`
	LatestRevisionNo int        `json:"latest_revision_no" db:"latest_revision_no"`
	LatestRevisionID *uuid.UUID `json:"latest_revision_id,omitempty" db:"latest_revision_id"`
	Files            []File     `json:"files,omitempty"`
	SourceDirectory  string     `json:"source_directory,omitempty" db:"source_path"`
	SourceCommit     string     `json:"source_commit,omitempty"`
}

type AppConfigRevision struct {
	ID           uuid.UUID `json:"id" db:"id"`
	AppConfigID  uuid.UUID `json:"app_config_id" db:"configuration_id"`
	RevisionNo   int       `json:"revision_no" db:"revision_no"`
	Files        []File    `json:"files" db:"files"`
	ContentHash  string    `json:"content_hash" db:"content_hash"`
	SourceCommit string    `json:"source_commit" db:"source_commit"`
	SourceDigest string    `json:"source_digest,omitempty" db:"source_digest"`
	CreatedAt    string    `json:"created_at,omitempty" db:"created_at"`
}

type AppConfigInput struct {
	ApplicationID uuid.UUID `json:"application_id"`
	EnvironmentID string    `json:"environment_id"`
	MountPath     string    `json:"mount_path,omitempty"`
}
