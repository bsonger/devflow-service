package service

import (
	"context"
	"database/sql"
	"errors"

	appdomain "github.com/bsonger/devflow-service/internal/application/domain"
	apprepo "github.com/bsonger/devflow-service/internal/application/repository"
	projectrepo "github.com/bsonger/devflow-service/internal/project/repository"
	"github.com/google/uuid"
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
	if err := s.syncProjectReference(ctx, application); err != nil {
		return uuid.Nil, err
	}
	return s.applications.Create(ctx, application)
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (*appdomain.Application, error) {
	return s.applications.Get(ctx, id)
}

func (s *service) Update(ctx context.Context, application *appdomain.Application) error {
	if err := s.syncProjectReference(ctx, application); err != nil {
		return err
	}
	return s.applications.Update(ctx, application)
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.applications.Delete(ctx, id)
}

func (s *service) List(ctx context.Context, filter appdomain.ListFilter) ([]appdomain.Application, error) {
	return s.applications.List(ctx, filter.IncludeDeleted, filter.Name, filter.ProjectID, filter.RepoAddress)
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
