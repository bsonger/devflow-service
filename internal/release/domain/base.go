package domain

import (
	"time"

	"github.com/google/uuid"
)

type BaseModel struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

func (b BaseModel) GetID() uuid.UUID { return b.ID }
func (b *BaseModel) SetID(id uuid.UUID) {
	b.ID = id
}

func (b *BaseModel) WithCreateDefault() {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	b.CreatedAt = time.Now()
	b.WithUpdateDefault()
}

func (b *BaseModel) WithUpdateDefault() {
	b.UpdatedAt = time.Now()
}
