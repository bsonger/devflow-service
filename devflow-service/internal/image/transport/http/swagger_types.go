package http

import (
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/google/uuid"
)

type ImageDoc struct {
	ID                      uuid.UUID       `json:"id"`
	ExecutionIntentID       *uuid.UUID      `json:"execution_intent_id,omitempty"`
	ApplicationID           uuid.UUID       `json:"application_id"`
	ConfigurationRevisionID *uuid.UUID      `json:"configuration_revision_id,omitempty"`
	RuntimeSpecRevisionID   *uuid.UUID      `json:"runtime_spec_revision_id,omitempty"`
	Name                    string          `json:"name"`
	Tag                     string          `json:"tag,omitempty"`
	Branch                  string          `json:"branch"`
	RepoAddress             string          `json:"repo_address"`
	CommitHash              string          `json:"commit_hash,omitempty"`
	Digest                  string          `json:"digest,omitempty"`
	PipelineID              string          `json:"pipeline_id,omitempty"`
	Steps                   []ImageTaskDoc  `json:"steps"`
	Status                  string          `json:"status"`
	CreatedAt               string          `json:"created_at,omitempty"`
	UpdatedAt               string          `json:"updated_at,omitempty"`
}

type ImageTaskDoc struct {
	TaskName  string  `json:"task_name"`
	TaskRun   string  `json:"task_run,omitempty"`
	Status    string  `json:"status"`
	StartTime string  `json:"start_time,omitempty"`
	EndTime   string  `json:"end_time,omitempty"`
	Message   string  `json:"message,omitempty"`
}

type ImageResponse struct {
	Data *ImageDoc `json:"data"`
}

type ImageListResponse struct {
	Data       []ImageDoc       `json:"data"`
	Pagination httpx.Pagination `json:"pagination"`
}
