package domain

import (
	"time"

	"github.com/google/uuid"
)

type Binding struct {
	ID            uuid.UUID  `json:"id" db:"binding_id"`
	ApplicationID uuid.UUID  `json:"application_id" db:"application_id"`
	EnvironmentID string     `json:"environment_id" db:"environment_id"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

type BindingInput struct {
	EnvironmentID string `json:"environment_id"`
}

func (b Binding) GetID() uuid.UUID { return b.ID }

func (b *Binding) SetID(id uuid.UUID) {
	b.ID = id
}

func (b *Binding) WithCreateDefault() {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	b.CreatedAt = time.Now()
	b.WithUpdateDefault()
}

func (b *Binding) WithUpdateDefault() {
	b.UpdatedAt = time.Now()
}
