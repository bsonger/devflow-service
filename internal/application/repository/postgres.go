package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	appdomain "github.com/bsonger/devflow-service/internal/application/domain"
	platformdb "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Store interface {
	Create(context.Context, *appdomain.Application) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*appdomain.Application, error)
	Update(context.Context, *appdomain.Application) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, bool, string, *uuid.UUID, string) ([]appdomain.Application, error)
}

var ApplicationStore Store = NewPostgresStore()

type postgresStore struct{}

func NewPostgresStore() Store {
	return &postgresStore{}
}

func (s *postgresStore) Create(ctx context.Context, app *appdomain.Application) (uuid.UUID, error) {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "create_application"),
		zap.String("resource", "application"),
	)

	labels, err := marshalLabels(app.Labels)
	if err != nil {
		log.Error("marshal application labels failed", zap.String("result", "error"), zap.Error(err))
		return uuid.Nil, err
	}

	_, err = platformdb.Postgres().ExecContext(ctx, `
		insert into applications (
			id, project_id, name, repo_address, description, labels, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, app.ID, dbsql.NullableUUID(app.ProjectID), app.Name, app.RepoAddress, app.Description, labels, app.CreatedAt, app.UpdatedAt, app.DeletedAt)
	if err != nil {
		log.Error("create application failed", zap.String("result", "error"), zap.Error(err))
		return uuid.Nil, err
	}

	log.Info("application created",
		zap.String("result", "success"),
		zap.String("resource_id", app.GetID().String()),
		zap.String("application_id", app.GetID().String()),
	)
	return app.GetID(), nil
}

func (s *postgresStore) Get(ctx context.Context, id uuid.UUID) (*appdomain.Application, error) {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "get_application"),
		zap.String("resource", "application"),
		zap.String("resource_id", id.String()),
		zap.String("application_id", id.String()),
	)

	app, err := scanApplication(platformdb.Postgres().QueryRowContext(ctx, `
		select id, project_id, name, repo_address, description, labels, created_at, updated_at, deleted_at
		from applications
		where id = $1 and deleted_at is null
	`, id))
	if err != nil {
		log.Error("get application failed", zap.String("result", "error"), zap.Error(err))
		return nil, err
	}

	log.Debug("application fetched",
		zap.String("result", "success"),
		zap.String("application_name", app.Name),
	)
	return app, nil
}

func (s *postgresStore) Update(ctx context.Context, app *appdomain.Application) error {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "update_application"),
		zap.String("resource", "application"),
		zap.String("resource_id", app.GetID().String()),
		zap.String("application_id", app.GetID().String()),
	)

	current, err := s.Get(ctx, app.GetID())
	if err != nil {
		log.Error("load application failed", zap.String("result", "error"), zap.Error(err))
		return err
	}

	app.CreatedAt = current.CreatedAt
	app.DeletedAt = current.DeletedAt
	app.WithUpdateDefault()

	labels, err := marshalLabels(app.Labels)
	if err != nil {
		return err
	}

	result, err := platformdb.Postgres().ExecContext(ctx, `
		update applications
		set project_id=$2, name=$3, repo_address=$4, description=$5, labels=$6, updated_at=$7, deleted_at=$8
		where id = $1 and deleted_at is null
	`, app.ID, dbsql.NullableUUID(app.ProjectID), app.Name, app.RepoAddress, app.Description, labels, app.UpdatedAt, app.DeletedAt)
	if err != nil {
		log.Error("update application failed", zap.String("result", "error"), zap.Error(err))
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	log.Debug("application updated",
		zap.String("result", "success"),
		zap.String("application_name", app.Name),
	)
	return nil
}

func (s *postgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "delete_application"),
		zap.String("resource", "application"),
		zap.String("resource_id", id.String()),
		zap.String("application_id", id.String()),
	)

	now := time.Now()
	result, err := platformdb.Postgres().ExecContext(ctx, `
		update applications
		set deleted_at=$2, updated_at=$2
		where id = $1 and deleted_at is null
	`, id, now)
	if err != nil {
		log.Error("delete application failed", zap.String("result", "error"), zap.Error(err))
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	log.Info("application deleted",
		zap.String("result", "success"),
		zap.String("resource", "application"),
		zap.String("resource_id", id.String()),
	)
	return nil
}

func (s *postgresStore) List(ctx context.Context, includeDeleted bool, name string, projectID *uuid.UUID, repoAddress string) ([]appdomain.Application, error) {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "list_applications"),
		zap.String("resource", "application"),
		zap.Bool("include_deleted", includeDeleted),
		zap.String("filter_name", name),
		zap.String("repo_address", repoAddress),
	)
	if projectID != nil {
		log = log.With(zap.String("filter_project_id", projectID.String()))
	}

	query := `
		select id, project_id, name, repo_address, description, labels, created_at, updated_at, deleted_at
		from applications
	`
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)

	if !includeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if name != "" {
		args = append(args, name)
		clauses = append(clauses, dbsql.PlaceholderClause("name", len(args)))
	}
	if projectID != nil {
		args = append(args, *projectID)
		clauses = append(clauses, dbsql.PlaceholderClause("project_id", len(args)))
	}
	if repoAddress != "" {
		args = append(args, repoAddress)
		clauses = append(clauses, dbsql.PlaceholderClause("repo_address", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"

	rows, err := platformdb.Postgres().QueryContext(ctx, query, args...)
	if err != nil {
		log.Error("list applications failed", zap.String("result", "error"), zap.Error(err))
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	apps := make([]appdomain.Application, 0)
	for rows.Next() {
		app, err := scanApplication(rows)
		if err != nil {
			return nil, err
		}
		apps = append(apps, *app)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	log.Debug("applications listed",
		zap.String("result", "success"),
		zap.Int("application_count", len(apps)),
	)
	return apps, nil
}

func scanApplication(scanner interface {
	Scan(dest ...any) error
}) (*appdomain.Application, error) {
	var (
		app         appdomain.Application
		projectID   sql.NullString
		labelsBytes []byte
		deletedAt   sql.NullTime
	)

	if err := scanner.Scan(
		&app.ID,
		&projectID,
		&app.Name,
		&app.RepoAddress,
		&app.Description,
		&labelsBytes,
		&app.CreatedAt,
		&app.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}

	projectUUID, err := dbsql.ParseNullUUID(projectID)
	if err != nil {
		return nil, err
	}
	if projectUUID != nil {
		app.ProjectID = *projectUUID
	}
	app.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	if len(labelsBytes) > 0 {
		labels, err := unmarshalLabels(labelsBytes)
		if err != nil {
			return nil, err
		}
		app.Labels = labels
	}

	return &app, nil
}

func marshalLabels(labels []appdomain.LabelItem) ([]byte, error) {
	return dbsql.MarshalLabelItems(labels)
}

func unmarshalLabels(raw []byte) ([]appdomain.LabelItem, error) {
	return dbsql.UnmarshalLabelItems(
		raw,
		func(key, value string) appdomain.LabelItem {
			return appdomain.LabelItem{Key: key, Value: value}
		},
		func(item appdomain.LabelItem) string {
			return item.Key
		},
	)
}
