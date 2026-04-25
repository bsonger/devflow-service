package domain

import (
	"time"

	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

type Intent struct {
	model.BaseModel

	Kind           model.IntentKind   `json:"kind" db:"kind"`
	Status         model.IntentStatus `json:"status" db:"status"`
	ResourceType   string             `json:"resource_type" db:"resource_type"`
	ResourceID     uuid.UUID          `json:"resource_id" db:"resource_id"`
	TraceID        string             `json:"trace_id,omitempty" db:"trace_id"`
	Message        string             `json:"message,omitempty" db:"message"`
	LastError      string             `json:"last_error,omitempty" db:"last_error"`
	ClaimedBy      string             `json:"claimed_by,omitempty" db:"claimed_by"`
	ClaimedAt      *time.Time         `json:"claimed_at,omitempty" db:"claimed_at"`
	LeaseExpiresAt *time.Time         `json:"lease_expires_at,omitempty" db:"lease_expires_at"`
	AttemptCount   int                `json:"attempt_count" db:"attempt_count"`
}

func (Intent) CollectionName() string { return "execution_intents" }
