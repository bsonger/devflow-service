package domain

type Project struct {
	BaseModel

	Name        string      `json:"name" db:"name"`
	Description string      `json:"description,omitempty" db:"description"`
	Labels      []LabelItem `json:"labels,omitempty" db:"labels"`
}

func (Project) CollectionName() string { return "projects" }
