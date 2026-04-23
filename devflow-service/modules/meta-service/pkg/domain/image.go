package domain

import "github.com/google/uuid"

type Image struct {
	BaseModel
	ApplicationID uuid.UUID `json:"application_id" db:"application_id"`
	Name          string    `json:"name" db:"name"`
}

func (Image) CollectionName() string { return "images" }
