package http

import "github.com/google/uuid"

type IntentDoc struct {
	ID             uuid.UUID `json:"id"`
	Kind           string    `json:"kind"`
	Status         string    `json:"status"`
	ResourceType   string    `json:"resource_type"`
	ResourceID     uuid.UUID `json:"resource_id"`
	TraceID        string    `json:"trace_id,omitempty"`
	Message        string    `json:"message,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	ClaimedBy      string    `json:"claimed_by,omitempty"`
	ClaimedAt      string    `json:"claimed_at,omitempty"`
	LeaseExpiresAt string    `json:"lease_expires_at,omitempty"`
	AttemptCount   int       `json:"attempt_count"`
	CreatedAt      string    `json:"created_at,omitempty"`
	UpdatedAt      string    `json:"updated_at,omitempty"`
}
