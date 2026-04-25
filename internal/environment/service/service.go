package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	clusterdomain "github.com/bsonger/devflow-service/internal/cluster/domain"
	clusterservice "github.com/bsonger/devflow-service/internal/cluster/service"
	envdomain "github.com/bsonger/devflow-service/internal/environment/domain"
	envrepo "github.com/bsonger/devflow-service/internal/environment/repository"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
)

var (
	ErrEnvironmentNameRequired    = sharederrs.Required("environment name")
	ErrEnvironmentClusterRequired = sharederrs.Required("cluster_id")
	ErrClusterReferenceNotFound   = sharederrs.InvalidArgument("cluster reference not found")
	ErrEnvironmentConflict        = sharederrs.Conflict("environment already exists")
)

type ListFilter struct {
	IncludeDeleted bool
	Name           string
	ClusterID      *uuid.UUID
}

type Service interface {
	Create(context.Context, *envdomain.Environment) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*envdomain.Environment, error)
	Update(context.Context, *envdomain.Environment) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, ListFilter) ([]envdomain.Environment, error)
}

var DefaultService Service = NewService(envrepo.EnvironmentStore, clusterservice.DefaultService)

type clusterReader interface {
	Get(context.Context, uuid.UUID) (*clusterdomain.Cluster, error)
}

type service struct {
	environments envrepo.Store
	clusters     clusterReader
}

func NewService(environments envrepo.Store, clusters clusterReader) Service {
	return &service{
		environments: environments,
		clusters:     clusters,
	}
}

func (s *service) Create(ctx context.Context, environment *envdomain.Environment) (uuid.UUID, error) {
	if err := validateEnvironment(environment); err != nil {
		return uuid.Nil, err
	}
	if err := s.syncClusterReference(ctx, environment); err != nil {
		return uuid.Nil, err
	}
	id, err := s.environments.Create(ctx, environment)
	if err != nil {
		if envrepo.AsPgConflict(err) {
			return uuid.Nil, ErrEnvironmentConflict
		}
		return uuid.Nil, err
	}
	return id, nil
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (*envdomain.Environment, error) {
	return s.environments.Get(ctx, id)
}

func (s *service) Update(ctx context.Context, environment *envdomain.Environment) error {
	if err := validateEnvironment(environment); err != nil {
		return err
	}
	if err := s.syncClusterReference(ctx, environment); err != nil {
		return err
	}
	if err := s.environments.Update(ctx, environment); err != nil {
		if envrepo.AsPgConflict(err) {
			return ErrEnvironmentConflict
		}
		return err
	}
	return nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.environments.Delete(ctx, id)
}

func (s *service) List(ctx context.Context, filter ListFilter) ([]envdomain.Environment, error) {
	return s.environments.List(ctx, filter.IncludeDeleted, filter.Name, filter.ClusterID)
}

func (s *service) syncClusterReference(ctx context.Context, environment *envdomain.Environment) error {
	if environment.ClusterID == uuid.Nil {
		return ErrEnvironmentClusterRequired
	}

	if _, err := s.clusters.Get(ctx, environment.ClusterID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrClusterReferenceNotFound
		}
		return err
	}
	return nil
}

func validateEnvironment(environment *envdomain.Environment) error {
	environment.Name = strings.TrimSpace(environment.Name)

	if environment.Name == "" {
		return ErrEnvironmentNameRequired
	}
	if environment.ClusterID == uuid.Nil {
		return ErrEnvironmentClusterRequired
	}
	return nil
}
