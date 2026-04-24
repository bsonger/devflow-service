package application

import (
	"context"

	appsvc "github.com/bsonger/devflow-service/internal/application/application"
	appdomain "github.com/bsonger/devflow-service/internal/application/domain"
	projectdomain "github.com/bsonger/devflow-service/internal/project/domain"
	projectrepo "github.com/bsonger/devflow-service/internal/project/repository"
	"github.com/google/uuid"
)

type ProjectListFilter struct {
	IncludeDeleted bool
	Name           string
}

type Service interface {
	Create(context.Context, *projectdomain.Project) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*projectdomain.Project, error)
	Update(context.Context, *projectdomain.Project) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, ProjectListFilter) ([]projectdomain.Project, error)
	ListApplications(context.Context, uuid.UUID) ([]projectdomain.Application, error)
}

var DefaultService Service = NewService(projectrepo.ProjectStore)

type service struct {
	store projectrepo.Store
}

func NewService(store projectrepo.Store) Service {
	return &service{store: store}
}

func (s *service) Create(ctx context.Context, project *projectdomain.Project) (uuid.UUID, error) {
	return s.store.Create(ctx, project)
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (*projectdomain.Project, error) {
	return s.store.Get(ctx, id)
}

func (s *service) Update(ctx context.Context, project *projectdomain.Project) error {
	return s.store.Update(ctx, project)
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.Delete(ctx, id)
}

func (s *service) List(ctx context.Context, filter ProjectListFilter) ([]projectdomain.Project, error) {
	return s.store.List(ctx, filter.IncludeDeleted, filter.Name)
}

func (s *service) ListApplications(ctx context.Context, projectID uuid.UUID) ([]projectdomain.Application, error) {
	if _, err := s.store.Get(ctx, projectID); err != nil {
		return nil, err
	}

	applications, err := appsvc.DefaultService.List(ctx, appsvc.ListFilter{ProjectID: &projectID})
	if err != nil {
		return nil, err
	}
	return toProjectApplications(applications), nil
}

func toProjectApplications(applications []appdomain.Application) []projectdomain.Application {
	out := make([]projectdomain.Application, 0, len(applications))
	for _, application := range applications {
		out = append(out, projectdomain.Application{
			BaseModel: projectdomain.BaseModel{
				ID:        application.ID,
				CreatedAt: application.CreatedAt,
				UpdatedAt: application.UpdatedAt,
				DeletedAt: application.DeletedAt,
			},
			ProjectID:     application.ProjectID,
			Name:          application.Name,
			RepoAddress:   application.RepoAddress,
			Description:   application.Description,
			ActiveImageID: application.ActiveImageID,
			Labels:        toProjectLabels(application.Labels),
		})
	}
	return out
}

func toProjectLabels(labels []appdomain.LabelItem) []projectdomain.LabelItem {
	if labels == nil {
		return nil
	}
	out := make([]projectdomain.LabelItem, 0, len(labels))
	for _, label := range labels {
		out = append(out, projectdomain.LabelItem{
			Key:   label.Key,
			Value: label.Value,
		})
	}
	return out
}
