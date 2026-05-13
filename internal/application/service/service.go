package service

import (
	"context"
	"database/sql"
	"errors"

	appdomain "github.com/bsonger/devflow-service/internal/application/domain"
	apprepo "github.com/bsonger/devflow-service/internal/application/repository"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	projectrepo "github.com/bsonger/devflow-service/internal/project/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service interface {
	Create(context.Context, *appdomain.Application) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*appdomain.Application, error)
	Update(context.Context, *appdomain.Application) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, appdomain.ListFilter) ([]appdomain.Application, error)
}

var DefaultService Service = NewService(apprepo.ApplicationStore, projectrepo.ProjectStore)

type service struct {
	applications apprepo.Store
	projects     projectrepo.Store
}

func NewService(applications apprepo.Store, projects projectrepo.Store) Service {
	return &service{
		applications: applications,
		projects:     projects,
	}
}

func (s *service) Create(ctx context.Context, application *appdomain.Application) (uuid.UUID, error) {
	log := platformobs.OperationLogger(ctx, "application_service", "create_application", "application",
		zap.String("devflow.project.id", applicationProjectID(application)),
	)
	if err := s.syncProjectReference(ctx, application); err != nil {
		platformobs.LogOperationFailure(log, "create application failed", err)
		return uuid.Nil, err
	}
	id, err := s.applications.Create(ctx, application)
	if err != nil {
		platformobs.LogOperationFailure(log, "create application failed", err)
		return uuid.Nil, err
	}
	platformobs.LogOperationSuccess(log, "application created",
		zap.String("resource_id", id.String()),
		zap.String("devflow.application.id", id.String()),
	)
	return id, nil
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (*appdomain.Application, error) {
	return s.applications.Get(ctx, id)
}

func (s *service) Update(ctx context.Context, application *appdomain.Application) error {
	resourceID := applicationID(application)
	log := platformobs.OperationLogger(ctx, "application_service", "update_application", "application",
		zap.String("resource_id", resourceID),
		zap.String("devflow.application.id", resourceID),
		zap.String("devflow.project.id", applicationProjectID(application)),
	)
	if err := s.syncProjectReference(ctx, application); err != nil {
		platformobs.LogOperationFailure(log, "update application failed", err)
		return err
	}
	if err := s.applications.Update(ctx, application); err != nil {
		platformobs.LogOperationFailure(log, "update application failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "application updated")
	return nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	log := platformobs.OperationLogger(ctx, "application_service", "delete_application", "application",
		zap.String("resource_id", id.String()),
		zap.String("devflow.application.id", id.String()),
	)
	if err := s.applications.Delete(ctx, id); err != nil {
		platformobs.LogOperationFailure(log, "delete application failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "application deleted")
	return nil
}

func (s *service) List(ctx context.Context, filter appdomain.ListFilter) ([]appdomain.Application, error) {
	return s.applications.List(ctx, filter.IncludeDeleted, filter.Name, filter.ProjectID, filter.RepoAddress)
}

func applicationID(application *appdomain.Application) string {
	if application == nil || application.ID == uuid.Nil {
		return ""
	}
	return application.ID.String()
}

func applicationProjectID(application *appdomain.Application) string {
	if application == nil || application.ProjectID == uuid.Nil {
		return ""
	}
	return application.ProjectID.String()
}

func (s *service) syncProjectReference(ctx context.Context, application *appdomain.Application) error {
	if application.ProjectID == uuid.Nil {
		return nil
	}
	if _, err := s.projects.Get(ctx, application.ProjectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appdomain.ErrProjectReferenceNotFound
		}
		return err
	}
	return nil
}
