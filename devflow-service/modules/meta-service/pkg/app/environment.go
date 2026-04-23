package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/modules/meta-service/pkg/domain"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/infra/store"
	"github.com/bsonger/devflow-service/shared/loggingx"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var EnvironmentService = NewEnvironmentService()

var (
	ErrEnvironmentNameRequired    = errors.New("environment name is required")
	ErrEnvironmentClusterRequired = errors.New("environment cluster_id is required")
	ErrClusterReferenceNotFound   = errors.New("cluster reference not found")
	ErrEnvironmentConflict        = errors.New("environment already exists")
)

type EnvironmentListFilter struct {
	IncludeDeleted bool
	Name           string
	ClusterID      *uuid.UUID
}

type environmentService struct{}

func NewEnvironmentService() *environmentService {
	return &environmentService{}
}

func (s *environmentService) Create(ctx context.Context, environment *domain.Environment) (uuid.UUID, error) {
	log := environmentLogger(ctx, "create_environment", environment.GetID(), environment.ClusterID)

	if err := validateEnvironment(environment); err != nil {
		logEnvironmentFailure(log, "create environment failed", err)
		return uuid.Nil, err
	}
	if err := s.syncClusterReference(ctx, environment); err != nil {
		logEnvironmentFailure(log, "resolve cluster reference failed", err)
		return uuid.Nil, err
	}

	labels, err := marshalLabels(environment.Labels)
	if err != nil {
		logEnvironmentFailure(log, "marshal environment labels failed", err)
		return uuid.Nil, err
	}

	_, err = store.DB().ExecContext(ctx, `
		insert into environments (
			id, name, cluster_id, description, labels, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8)
	`, environment.ID, environment.Name, environment.ClusterID, environment.Description, labels, environment.CreatedAt, environment.UpdatedAt, environment.DeletedAt)
	if err != nil {
		if isUniqueViolation(err) {
			logEnvironmentFailure(log, "create environment conflict", ErrEnvironmentConflict)
			return uuid.Nil, ErrEnvironmentConflict
		}
		logEnvironmentFailure(log, "create environment failed", err)
		return uuid.Nil, err
	}

	log.Info("environment created",
		zap.String("result", "success"),
		zap.String("environment_name", environment.Name),
	)
	return environment.GetID(), nil
}

func (s *environmentService) Get(ctx context.Context, id uuid.UUID) (*domain.Environment, error) {
	log := environmentLogger(ctx, "get_environment", id, uuid.Nil)

	environment, err := scanEnvironment(store.DB().QueryRowContext(ctx, `
		select id, name, cluster_id, description, labels, created_at, updated_at, deleted_at
		from environments
		where id = $1 and deleted_at is null
	`, id))
	if err != nil {
		logEnvironmentFailure(log, "get environment failed", err)
		return nil, err
	}

	log.Debug("environment fetched",
		zap.String("result", "success"),
		zap.String("environment_name", environment.Name),
		zap.String("cluster_id", environment.ClusterID.String()),
	)
	return environment, nil
}

func (s *environmentService) Update(ctx context.Context, environment *domain.Environment) error {
	log := environmentLogger(ctx, "update_environment", environment.GetID(), environment.ClusterID)

	current, err := s.Get(ctx, environment.GetID())
	if err != nil {
		logEnvironmentFailure(log, "load environment failed", err)
		return err
	}

	environment.CreatedAt = current.CreatedAt
	environment.DeletedAt = current.DeletedAt
	environment.WithUpdateDefault()

	if err := validateEnvironment(environment); err != nil {
		logEnvironmentFailure(log, "update environment failed", err)
		return err
	}
	if err := s.syncClusterReference(ctx, environment); err != nil {
		logEnvironmentFailure(log, "resolve cluster reference failed", err)
		return err
	}

	labels, err := marshalLabels(environment.Labels)
	if err != nil {
		logEnvironmentFailure(log, "marshal environment labels failed", err)
		return err
	}

	result, err := store.DB().ExecContext(ctx, `
		update environments
		set name=$2, cluster_id=$3, description=$4, labels=$5, updated_at=$6, deleted_at=$7
		where id = $1 and deleted_at is null
	`, environment.ID, environment.Name, environment.ClusterID, environment.Description, labels, environment.UpdatedAt, environment.DeletedAt)
	if err != nil {
		if isUniqueViolation(err) {
			logEnvironmentFailure(log, "update environment conflict", ErrEnvironmentConflict)
			return ErrEnvironmentConflict
		}
		logEnvironmentFailure(log, "update environment failed", err)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		logEnvironmentFailure(log, "read environment update result failed", err)
		return err
	}
	if rows == 0 {
		logEnvironmentFailure(log, "update environment missed row", sql.ErrNoRows)
		return sql.ErrNoRows
	}

	log.Info("environment updated",
		zap.String("result", "success"),
		zap.String("environment_name", environment.Name),
		zap.String("cluster_id", environment.ClusterID.String()),
	)
	return nil
}

func (s *environmentService) Delete(ctx context.Context, id uuid.UUID) error {
	log := environmentLogger(ctx, "delete_environment", id, uuid.Nil)

	now := time.Now()
	result, err := store.DB().ExecContext(ctx, `
		update environments
		set deleted_at=$2, updated_at=$2
		where id = $1 and deleted_at is null
	`, id, now)
	if err != nil {
		logEnvironmentFailure(log, "delete environment failed", err)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		logEnvironmentFailure(log, "read environment delete result failed", err)
		return err
	}
	if rows == 0 {
		logEnvironmentFailure(log, "delete environment missed row", sql.ErrNoRows)
		return sql.ErrNoRows
	}

	log.Info("environment deleted", zap.String("result", "success"))
	return nil
}

func (s *environmentService) List(ctx context.Context, filter EnvironmentListFilter) ([]domain.Environment, error) {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "list_environments"),
		zap.String("resource", "environment"),
		zap.String("result", "started"),
		zap.Any("filter", filter),
	)

	query := `
		select id, name, cluster_id, description, labels, created_at, updated_at, deleted_at
		from environments
	`
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)

	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.Name != "" {
		args = append(args, strings.TrimSpace(filter.Name))
		clauses = append(clauses, placeholderClause("name", len(args)))
	}
	if filter.ClusterID != nil {
		args = append(args, *filter.ClusterID)
		clauses = append(clauses, placeholderClause("cluster_id", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"

	rows, err := store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		logEnvironmentFailure(log, "list environments failed", err)
		return nil, err
	}
	defer rows.Close()

	environments := make([]domain.Environment, 0)
	for rows.Next() {
		environment, err := scanEnvironment(rows)
		if err != nil {
			logEnvironmentFailure(log, "scan environment failed", err)
			return nil, err
		}
		environments = append(environments, *environment)
	}
	if err := rows.Err(); err != nil {
		logEnvironmentFailure(log, "iterate environments failed", err)
		return nil, err
	}

	log.Debug("environments listed", zap.String("result", "success"), zap.Int("count", len(environments)))
	return environments, nil
}

func (s *environmentService) syncClusterReference(ctx context.Context, environment *domain.Environment) error {
	if environment.ClusterID == uuid.Nil {
		return ErrEnvironmentClusterRequired
	}

	if _, err := ClusterService.Get(ctx, environment.ClusterID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrClusterReferenceNotFound
		}
		return err
	}
	return nil
}

func validateEnvironment(environment *domain.Environment) error {
	environment.Name = strings.TrimSpace(environment.Name)

	if environment.Name == "" {
		return ErrEnvironmentNameRequired
	}
	if environment.ClusterID == uuid.Nil {
		return ErrEnvironmentClusterRequired
	}
	return nil
}

func scanEnvironment(scanner interface {
	Scan(dest ...any) error
}) (*domain.Environment, error) {
	var (
		environment domain.Environment
		clusterID   sql.NullString
		labels      []byte
		deletedAt   sql.NullTime
	)

	if err := scanner.Scan(
		&environment.ID,
		&environment.Name,
		&clusterID,
		&environment.Description,
		&labels,
		&environment.CreatedAt,
		&environment.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}

	if clusterID.Valid {
		parsed, err := uuid.Parse(clusterID.String)
		if err != nil {
			return nil, err
		}
		environment.ClusterID = parsed
	}
	if deletedAt.Valid {
		environment.DeletedAt = &deletedAt.Time
	}
	if len(labels) > 0 {
		parsed, err := unmarshalLabels(labels)
		if err != nil {
			return nil, err
		}
		environment.Labels = parsed
	}

	return &environment, nil
}

func environmentLogger(ctx context.Context, operation string, resourceID, clusterID uuid.UUID) *zap.Logger {
	resourceIDValue := ""
	if resourceID != uuid.Nil {
		resourceIDValue = resourceID.String()
	}
	clusterIDValue := ""
	if clusterID != uuid.Nil {
		clusterIDValue = clusterID.String()
	}
	return loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", operation),
		zap.String("resource", "environment"),
		zap.String("resource_id", resourceIDValue),
		zap.String("error_code", ""),
		zap.String("cluster_id", clusterIDValue),
	)
}

func logEnvironmentFailure(log *zap.Logger, msg string, err error) {
	log.Error(msg,
		zap.String("result", "error"),
		zap.String("error_code", appErrorCode(err)),
		zap.Error(err),
	)
}
