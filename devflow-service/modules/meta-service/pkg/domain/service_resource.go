package domain

import "github.com/google/uuid"

type ServiceResource struct {
	BaseModel

	ApplicationID uuid.UUID     `json:"application_id,omitempty" db:"application_id"`
	Name          string        `json:"name" db:"name"`
	Description   string        `json:"description,omitempty" db:"description"`
	Labels        []LabelItem   `json:"labels,omitempty" db:"labels"`
	Ports         []ServicePort `json:"ports" db:"ports"`
}

func (ServiceResource) CollectionName() string { return "services" }
