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
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
	"go.uber.org/zap"
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
	log := platformobs.OperationLogger(ctx, "environment_service", "create_environment", "environment",
		zap.String("cluster_id", environmentClusterID(environment)),
	)
	if err := validateEnvironment(environment); err != nil {
		platformobs.LogOperationFailure(log, "create environment failed", err)
		return uuid.Nil, err
	}
	if err := s.syncClusterReference(ctx, environment); err != nil {
		platformobs.LogOperationFailure(log, "create environment failed", err)
		return uuid.Nil, err
	}
	id, err := s.environments.Create(ctx, environment)
	if err != nil {
		if envrepo.AsPgConflict(err) {
			platformobs.LogOperationFailure(log, "create environment failed", ErrEnvironmentConflict)
			return uuid.Nil, ErrEnvironmentConflict
		}
		platformobs.LogOperationFailure(log, "create environment failed", err)
		return uuid.Nil, err
	}
	platformobs.LogOperationSuccess(log, "environment created",
		zap.String("resource_id", id.String()),
		zap.String("devflow.environment.id", id.String()),
	)
	return id, nil
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (*envdomain.Environment, error) {
	return s.environments.Get(ctx, id)
}

func (s *service) Update(ctx context.Context, environment *envdomain.Environment) error {
	log := platformobs.OperationLogger(ctx, "environment_service", "update_environment", "environment",
		zap.String("resource_id", environmentID(environment)),
		zap.String("devflow.environment.id", environmentID(environment)),
		zap.String("cluster_id", environmentClusterID(environment)),
	)
	if err := validateEnvironment(environment); err != nil {
		platformobs.LogOperationFailure(log, "update environment failed", err)
		return err
	}
	if err := s.syncClusterReference(ctx, environment); err != nil {
		platformobs.LogOperationFailure(log, "update environment failed", err)
		return err
	}
	if err := s.environments.Update(ctx, environment); err != nil {
		if envrepo.AsPgConflict(err) {
			platformobs.LogOperationFailure(log, "update environment failed", ErrEnvironmentConflict)
			return ErrEnvironmentConflict
		}
		platformobs.LogOperationFailure(log, "update environment failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "environment updated")
	return nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	log := platformobs.OperationLogger(ctx, "environment_service", "delete_environment", "environment",
		zap.String("resource_id", id.String()),
		zap.String("devflow.environment.id", id.String()),
	)
	if err := s.environments.Delete(ctx, id); err != nil {
		platformobs.LogOperationFailure(log, "delete environment failed", err)
		return err
	}
	platformobs.LogOperationSuccess(log, "environment deleted")
	return nil
}

func (s *service) List(ctx context.Context, filter ListFilter) ([]envdomain.Environment, error) {
	return s.environments.List(ctx, filter.IncludeDeleted, filter.Name, filter.ClusterID)
}

func environmentID(environment *envdomain.Environment) string {
	if environment == nil || environment.ID == uuid.Nil {
		return ""
	}
	return environment.ID.String()
}

func environmentClusterID(environment *envdomain.Environment) string {
	if environment == nil || environment.ClusterID == uuid.Nil {
		return ""
	}
	return environment.ClusterID.String()
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
