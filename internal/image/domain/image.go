package domain

import (
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

type Image struct {
	model.BaseModel

	ExecutionIntentID       *uuid.UUID        `json:"execution_intent_id,omitempty" db:"execution_intent_id"`
	ApplicationID           uuid.UUID         `json:"application_id" db:"application_id"`
	ConfigurationRevisionID *uuid.UUID        `json:"configuration_revision_id,omitempty" db:"configuration_revision_id"`
	RuntimeSpecRevisionID   *uuid.UUID        `json:"runtime_spec_revision_id,omitempty" db:"runtime_spec_revision_id"`
	Name                    string            `json:"name" db:"name"`
	Tag                     string            `json:"tag,omitempty" db:"tag"`
	Branch                  string            `json:"branch" db:"branch"`
	RepoAddress             string            `json:"repo_address" db:"repo_address"`
	CommitHash              string            `json:"commit_hash,omitempty" db:"commit_hash"`
	Digest                  string            `json:"digest,omitempty" db:"digest"`
	PipelineID              string            `json:"pipeline_id,omitempty" db:"pipeline_id"`
	Steps                   []model.ImageTask `json:"steps" db:"steps"`
	Status                  model.ImageStatus `json:"status" db:"status"`
}

type CreateImageRequest struct {
	ApplicationID           uuid.UUID  `json:"application_id"`
	ConfigurationRevisionID *uuid.UUID `json:"configuration_revision_id,omitempty"`
	RuntimeSpecRevisionID   *uuid.UUID `json:"runtime_spec_revision_id,omitempty"`
	Branch                  string     `json:"branch,omitempty"`
}

type PatchImageRequest struct {
	CommitHash string `json:"commit_hash,omitempty"`
	Digest     string `json:"digest,omitempty"`
	Tag        string `json:"tag,omitempty"`
}

func (r *PatchImageRequest) IsEmpty() bool {
	return r.CommitHash == "" && r.Digest == "" && r.Tag == ""
}

func (m *Image) CollectionName() string { return "images" }
