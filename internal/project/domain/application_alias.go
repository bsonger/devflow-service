package domain

import "github.com/google/uuid"

type Application struct {
	BaseModel

	ProjectID   uuid.UUID   `json:"project_id" db:"project_id"`
	Name        string      `json:"name" db:"name"`
	RepoAddress string      `json:"repo_address" db:"repo_address"`
	Description string      `json:"description,omitempty" db:"description"`
	Labels      []LabelItem `json:"labels,omitempty" db:"labels"`
}
