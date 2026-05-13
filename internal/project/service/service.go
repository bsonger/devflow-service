package service

import (
	"context"

	appdomain "github.com/bsonger/devflow-service/internal/application/domain"
	applicationservice "github.com/bsonger/devflow-service/internal/application/service"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	projectdomain "github.com/bsonger/devflow-service/internal/project/domain"
	projectrepo "github.com/bsonger/devflow-service/internal/project/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
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
	log := platformobs.OperationLogger(ctx, "project_service", "create_project", "project")
	id, err := s.store.Create(ctx, project)
	if err != nil {
		platformobs.LogOperationFailure(log, "create project failed", err)
		return uuid.Nil, err
	}
	platformobs.LogOperationSuccess(log, "project created",
		zap.String("resource_id", id.String()),
		zap.String("devflow.project.id", id.String()),
	)
	return id, nil
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (*projectdomain.Project, error) {
	return s.store.Get(ctx, id)
}

func (s *service) Update(ctx context.Context, project *projectdomain.Project) error {
	log := platformobs.OperationLogger(ctx, "project_service", "update_project", "project",
		zap.String("resource_id", projectID(project)),
		zap.String("devflow.project.id", projectID(project)),
	)
	if err := s.store.Update(ctx, project); err != nil {
		platformobs.LogOperationFailure(log, "update project failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "project updated")
	return nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	log := platformobs.OperationLogger(ctx, "project_service", "delete_project", "project",
		zap.String("resource_id", id.String()),
		zap.String("devflow.project.id", id.String()),
	)
	if err := s.store.Delete(ctx, id); err != nil {
		platformobs.LogOperationFailure(log, "delete project failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "project deleted")
	return nil
}

func (s *service) List(ctx context.Context, filter ProjectListFilter) ([]projectdomain.Project, error) {
	return s.store.List(ctx, filter.IncludeDeleted, filter.Name)
}

func projectID(project *projectdomain.Project) string {
	if project == nil || project.ID == uuid.Nil {
		return ""
	}
	return project.ID.String()
}

func (s *service) ListApplications(ctx context.Context, projectID uuid.UUID) ([]projectdomain.Application, error) {
	if _, err := s.store.Get(ctx, projectID); err != nil {
		return nil, err
	}

	applications, err := applicationservice.DefaultService.List(ctx, appdomain.ListFilter{ProjectID: &projectID})
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
			ProjectID:   application.ProjectID,
			Name:        application.Name,
			RepoAddress: application.RepoAddress,
			Description: application.Description,
			Labels:      toProjectLabels(application.Labels),
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
