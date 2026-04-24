package application

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	clusterdomain "github.com/bsonger/devflow-service/internal/cluster/domain"
	clusterrepo "github.com/bsonger/devflow-service/internal/cluster/repository"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ErrClusterNameRequired         = errors.New("cluster name is required")
	ErrClusterServerRequired       = errors.New("cluster server is required")
	ErrClusterKubeConfigRequired   = errors.New("cluster kubeconfig is required")
	ErrClusterConflict             = errors.New("cluster already exists")
	ErrClusterOnboardingFailed     = errors.New("cluster onboarding failed")
	ErrClusterOnboardingTimeout    = errors.New("cluster onboarding timed out")
	ErrClusterOnboardingMalformed  = errors.New("cluster onboarding payload malformed")
	ErrClusterOnboardingStatusSync = errors.New("cluster onboarding status persistence failed")
)

type ListFilter struct {
	IncludeDeleted bool
	Name           string
}

type Service interface {
	Create(context.Context, *clusterdomain.Cluster) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*clusterdomain.Cluster, error)
	Update(context.Context, *clusterdomain.Cluster) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, ListFilter) ([]clusterdomain.Cluster, error)
}

var DefaultService Service = NewService(clusterrepo.ClusterStore, newKubernetesClusterOnboarder(), time.Now)

type service struct {
	store      clusterrepo.Store
	onboarding clusterOnboardingExecutor
	now        func() time.Time
}

func NewService(store clusterrepo.Store, onboarding clusterOnboardingExecutor, now func() time.Time) Service {
	if onboarding == nil {
		onboarding = noopClusterOnboardingExecutor{}
	}
	if now == nil {
		now = time.Now
	}
	return &service{store: store, onboarding: onboarding, now: now}
}

func (s *service) Create(ctx context.Context, cluster *clusterdomain.Cluster) (uuid.UUID, error) {
	log := clusterLogger(ctx, "create_cluster", cluster.GetID())

	if err := validateCluster(cluster); err != nil {
		logClusterFailure(log, "create cluster failed", err)
		return uuid.Nil, err
	}

	_, err := s.store.Create(ctx, cluster)
	if err != nil {
		if clusterrepo.AsPgConflict(err) {
			logClusterFailure(log, "create cluster conflict", ErrClusterConflict)
			return uuid.Nil, ErrClusterConflict
		}
		logClusterFailure(log, "create cluster failed", err)
		return uuid.Nil, err
	}

	onboardingErr := s.runClusterOnboarding(ctx, cluster)
	if onboardingErr != nil {
		logClusterFailure(log, "cluster onboarding failed", onboardingErr)
		return uuid.Nil, onboardingErr
	}

	log.Info("cluster created",
		zap.String("result", "success"),
		zap.String("cluster_name", cluster.Name),
		zap.String("server", cluster.Server),
		zap.Bool("onboarding_ready", cluster.OnboardingReady),
	)
	return cluster.GetID(), nil
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (*clusterdomain.Cluster, error) {
	return s.store.Get(ctx, id)
}

func (s *service) Update(ctx context.Context, cluster *clusterdomain.Cluster) error {
	log := clusterLogger(ctx, "update_cluster", cluster.GetID())

	current, err := s.store.Get(ctx, cluster.GetID())
	if err != nil {
		logClusterFailure(log, "load cluster failed", err)
		return err
	}

	cluster.CreatedAt = current.CreatedAt
	cluster.DeletedAt = current.DeletedAt
	cluster.WithUpdateDefault()

	if err := validateCluster(cluster); err != nil {
		logClusterFailure(log, "update cluster failed", err)
		return err
	}

	err = s.store.Update(ctx, cluster)
	if err != nil {
		if clusterrepo.AsPgConflict(err) {
			logClusterFailure(log, "update cluster conflict", ErrClusterConflict)
			return ErrClusterConflict
		}
		logClusterFailure(log, "update cluster failed", err)
		return err
	}

	cluster.OnboardingReady = current.OnboardingReady
	cluster.OnboardingError = current.OnboardingError
	cluster.OnboardingCheckedAt = current.OnboardingCheckedAt
	if requiresClusterOnboarding(current, cluster) {
		onboardingErr := s.runClusterOnboarding(ctx, cluster)
		if onboardingErr != nil {
			logClusterFailure(log, "cluster onboarding failed", onboardingErr)
			return onboardingErr
		}
	}

	log.Info("cluster updated",
		zap.String("result", "success"),
		zap.String("cluster_name", cluster.Name),
		zap.String("server", cluster.Server),
		zap.Bool("onboarding_ready", cluster.OnboardingReady),
	)
	return nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.Delete(ctx, id)
}

func (s *service) List(ctx context.Context, filter ListFilter) ([]clusterdomain.Cluster, error) {
	return s.store.List(ctx, filter.IncludeDeleted, filter.Name)
}

func (s *service) runClusterOnboarding(ctx context.Context, cluster *clusterdomain.Cluster) error {
	checkedAt := s.now().UTC()
	onboardingErr := normalizeOnboardingError(s.onboarding.Onboard(ctx, cluster))

	status := clusterOnboardingStatusFromResult(onboardingErr, checkedAt)
	if persistErr := s.persistOnboardingStatus(ctx, cluster.ID, status); persistErr != nil {
		return persistErr
	}

	cluster.OnboardingReady = status.Ready
	cluster.OnboardingError = status.Error
	cluster.OnboardingCheckedAt = &status.CheckedAt
	return onboardingErr
}

func (s *service) persistOnboardingStatus(ctx context.Context, clusterID uuid.UUID, status clusterOnboardingStatus) error {
	if clusterID == uuid.Nil {
		return ErrClusterOnboardingStatusSync
	}
	if err := s.store.PersistOnboardingStatus(ctx, clusterID, status.Ready, status.Error, status.CheckedAt, s.now()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return ErrClusterOnboardingStatusSync
	}
	return nil
}

func validateCluster(cluster *clusterdomain.Cluster) error {
	cluster.Name = strings.TrimSpace(cluster.Name)
	cluster.Server = strings.TrimSpace(cluster.Server)
	cluster.ArgoCDClusterName = strings.TrimSpace(cluster.ArgoCDClusterName)

	if cluster.Name == "" {
		return ErrClusterNameRequired
	}
	if cluster.Server == "" {
		return ErrClusterServerRequired
	}
	if strings.TrimSpace(cluster.KubeConfig) == "" {
		return ErrClusterKubeConfigRequired
	}
	return nil
}

func clusterLogger(ctx context.Context, operation string, resourceID uuid.UUID) *zap.Logger {
	resourceIDValue := ""
	if resourceID != uuid.Nil {
		resourceIDValue = resourceID.String()
	}
	return logger.LoggerWithContext(ctx).With(
		zap.String("operation", operation),
		zap.String("resource", "cluster"),
		zap.String("resource_id", resourceIDValue),
	)
}

func logClusterFailure(log *zap.Logger, msg string, err error) {
	log.Error(msg,
		zap.String("result", "error"),
		zap.String("error_code", appErrorCode(err)),
		zap.Error(err),
	)
}

func appErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, sql.ErrNoRows):
		return "not_found"
	case errors.Is(err, ErrClusterConflict):
		return "conflict"
	case errors.Is(err, ErrClusterNameRequired),
		errors.Is(err, ErrClusterServerRequired),
		errors.Is(err, ErrClusterKubeConfigRequired),
		errors.Is(err, ErrClusterOnboardingMalformed):
		return "invalid_argument"
	case errors.Is(err, ErrClusterOnboardingTimeout):
		return "deadline_exceeded"
	case errors.Is(err, ErrClusterOnboardingFailed):
		return "failed_precondition"
	default:
		return "internal"
	}
}
