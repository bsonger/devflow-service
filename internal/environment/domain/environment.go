package domain

import "github.com/google/uuid"

type Environment struct {
	BaseModel

	Name        string      `json:"name" db:"name"`
	ClusterID   uuid.UUID   `json:"cluster_id" db:"cluster_id"`
	Description string      `json:"description,omitempty" db:"description"`
	Labels      []LabelItem `json:"labels,omitempty" db:"labels"`
}

func (Environment) CollectionName() string { return "environments" }
