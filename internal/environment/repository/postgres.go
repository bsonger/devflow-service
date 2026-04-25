package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	envdomain "github.com/bsonger/devflow-service/internal/environment/domain"
	platformdb "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

type Store interface {
	Create(context.Context, *envdomain.Environment) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*envdomain.Environment, error)
	Update(context.Context, *envdomain.Environment) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, bool, string, *uuid.UUID) ([]envdomain.Environment, error)
}

var EnvironmentStore Store = NewPostgresStore()

type postgresStore struct{}

func NewPostgresStore() Store {
	return &postgresStore{}
}

func (s *postgresStore) Create(ctx context.Context, environment *envdomain.Environment) (uuid.UUID, error) {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "create_environment"),
		zap.String("resource", "environment"),
		zap.String("cluster_id", environment.ClusterID.String()),
	)

	labels, err := marshalLabels(environment.Labels)
	if err != nil {
		logFailure(log, "marshal environment labels failed", err)
		return uuid.Nil, err
	}

	_, err = platformdb.Postgres().ExecContext(ctx, `
		insert into environments (
			id, name, cluster_id, description, labels, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8)
	`, environment.ID, environment.Name, environment.ClusterID, environment.Description, labels, environment.CreatedAt, environment.UpdatedAt, environment.DeletedAt)
	if err != nil {
		logFailure(log, "create environment failed", err)
		return uuid.Nil, err
	}

	log.Info("environment created",
		zap.String("result", "success"),
		zap.String("resource_id", environment.GetID().String()),
		zap.String("environment_name", environment.Name),
	)
	return environment.GetID(), nil
}

func (s *postgresStore) Get(ctx context.Context, id uuid.UUID) (*envdomain.Environment, error) {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "get_environment"),
		zap.String("resource", "environment"),
		zap.String("resource_id", id.String()),
	)

	environment, err := scanEnvironment(platformdb.Postgres().QueryRowContext(ctx, `
		select id, name, cluster_id, description, labels, created_at, updated_at, deleted_at
		from environments
		where id = $1 and deleted_at is null
	`, id))
	if err != nil {
		logFailure(log, "get environment failed", err)
		return nil, err
	}

	log.Debug("environment fetched",
		zap.String("result", "success"),
		zap.String("environment_name", environment.Name),
		zap.String("cluster_id", environment.ClusterID.String()),
	)
	return environment, nil
}

func (s *postgresStore) Update(ctx context.Context, environment *envdomain.Environment) error {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "update_environment"),
		zap.String("resource", "environment"),
		zap.String("resource_id", environment.GetID().String()),
		zap.String("cluster_id", environment.ClusterID.String()),
	)

	current, err := s.Get(ctx, environment.GetID())
	if err != nil {
		logFailure(log, "load environment failed", err)
		return err
	}

	environment.CreatedAt = current.CreatedAt
	environment.DeletedAt = current.DeletedAt
	environment.WithUpdateDefault()

	labels, err := marshalLabels(environment.Labels)
	if err != nil {
		logFailure(log, "marshal environment labels failed", err)
		return err
	}

	result, err := platformdb.Postgres().ExecContext(ctx, `
		update environments
		set name=$2, cluster_id=$3, description=$4, labels=$5, updated_at=$6, deleted_at=$7
		where id = $1 and deleted_at is null
	`, environment.ID, environment.Name, environment.ClusterID, environment.Description, labels, environment.UpdatedAt, environment.DeletedAt)
	if err != nil {
		logFailure(log, "update environment failed", err)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		logFailure(log, "read environment update result failed", err)
		return err
	}
	if rows == 0 {
		logFailure(log, "update environment missed row", sql.ErrNoRows)
		return sql.ErrNoRows
	}

	log.Info("environment updated",
		zap.String("result", "success"),
		zap.String("resource_id", environment.GetID().String()),
		zap.String("environment_name", environment.Name),
		zap.String("cluster_id", environment.ClusterID.String()),
	)
	return nil
}

func (s *postgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "delete_environment"),
		zap.String("resource", "environment"),
		zap.String("resource_id", id.String()),
	)

	now := time.Now()
	result, err := platformdb.Postgres().ExecContext(ctx, `
		update environments
		set deleted_at=$2, updated_at=$2
		where id = $1 and deleted_at is null
	`, id, now)
	if err != nil {
		logFailure(log, "delete environment failed", err)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		logFailure(log, "read environment delete result failed", err)
		return err
	}
	if rows == 0 {
		logFailure(log, "delete environment missed row", sql.ErrNoRows)
		return sql.ErrNoRows
	}

	log.Info("environment deleted",
		zap.String("result", "success"),
		zap.String("resource_id", id.String()),
	)
	return nil
}

func (s *postgresStore) List(ctx context.Context, includeDeleted bool, name string, clusterID *uuid.UUID) ([]envdomain.Environment, error) {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "list_environments"),
		zap.String("resource", "environment"),
		zap.Bool("include_deleted", includeDeleted),
		zap.String("filter_name", name),
	)
	if clusterID != nil {
		log = log.With(zap.String("filter_cluster_id", clusterID.String()))
	}

	query := `
		select id, name, cluster_id, description, labels, created_at, updated_at, deleted_at
		from environments
	`
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)

	if !includeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if name != "" {
		args = append(args, strings.TrimSpace(name))
		clauses = append(clauses, dbsql.PlaceholderClause("name", len(args)))
	}
	if clusterID != nil {
		args = append(args, *clusterID)
		clauses = append(clauses, dbsql.PlaceholderClause("cluster_id", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"

	rows, err := platformdb.Postgres().QueryContext(ctx, query, args...)
	if err != nil {
		logFailure(log, "list environments failed", err)
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	environments := make([]envdomain.Environment, 0)
	for rows.Next() {
		environment, err := scanEnvironment(rows)
		if err != nil {
			logFailure(log, "scan environment failed", err)
			return nil, err
		}
		environments = append(environments, *environment)
	}
	if err := rows.Err(); err != nil {
		logFailure(log, "iterate environments failed", err)
		return nil, err
	}

	log.Debug("environments listed", zap.String("result", "success"), zap.Int("environment_count", len(environments)))
	return environments, nil
}

func scanEnvironment(scanner interface {
	Scan(dest ...any) error
}) (*envdomain.Environment, error) {
	var (
		environment envdomain.Environment
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

	clusterUUID, err := dbsql.ParseNullUUID(clusterID)
	if err != nil {
		return nil, err
	}
	if clusterUUID != nil {
		environment.ClusterID = *clusterUUID
	}
	environment.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	if len(labels) > 0 {
		parsed, err := unmarshalLabels(labels)
		if err != nil {
			return nil, err
		}
		environment.Labels = parsed
	}

	return &environment, nil
}

func marshalLabels(labels []envdomain.LabelItem) ([]byte, error) {
	return dbsql.MarshalLabelItems(labels)
}

func unmarshalLabels(raw []byte) ([]envdomain.LabelItem, error) {
	return dbsql.UnmarshalLabelItems(
		raw,
		func(key, value string) envdomain.LabelItem {
			return envdomain.LabelItem{Key: key, Value: value}
		},
		func(item envdomain.LabelItem) string {
			return item.Key
		},
	)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return err != nil && sql.ErrNoRows != err && errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func AsPgConflict(err error) bool {
	return isUniqueViolation(err)
}

func appErrorCode(err error) string {
	switch err {
	case nil:
		return ""
	case sql.ErrNoRows:
		return "not_found"
	default:
		return "internal"
	}
}

func logFailure(log *zap.Logger, msg string, err error) {
	log.Error(msg,
		zap.String("result", "error"),
		zap.String("error_code", appErrorCode(err)),
		zap.Error(err),
	)
}
