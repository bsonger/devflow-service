package domain

import "github.com/google/uuid"

type ListFilter struct {
	IncludeDeleted bool
	Name           string
	ProjectID      *uuid.UUID
	RepoAddress    string
}
