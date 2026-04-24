package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	appdomain "github.com/bsonger/devflow-service/internal/application/domain"
	platformdb "github.com/bsonger/devflow-service/internal/platform/db"
	loggingx "github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Store interface {
	Create(context.Context, *appdomain.Application) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*appdomain.Application, error)
	Update(context.Context, *appdomain.Application) error
	Delete(context.Context, uuid.UUID) error
	UpdateActiveImage(context.Context, uuid.UUID, uuid.UUID) error
	List(context.Context, bool, string, *uuid.UUID, string) ([]appdomain.Application, error)
}

var ApplicationStore Store = NewPostgresStore()

type postgresStore struct{}

func NewPostgresStore() Store {
	return &postgresStore{}
}

func (s *postgresStore) Create(ctx context.Context, app *appdomain.Application) (uuid.UUID, error) {
	log := loggingx.LoggerWithContext(ctx).With(zap.String("operation", "create_application"))

	labels, err := marshalLabels(app.Labels)
	if err != nil {
		log.Error("marshal application labels failed", zap.Error(err))
		return uuid.Nil, err
	}

	_, err = platformdb.Postgres().ExecContext(ctx, `
		insert into applications (
			id, project_id, name, repo_address, description, active_image_id, labels, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, app.ID, nullableUUID(app.ProjectID), app.Name, app.RepoAddress, app.Description, nullableUUIDPtr(app.ActiveImageID), labels, app.CreatedAt, app.UpdatedAt, app.DeletedAt)
	if err != nil {
		log.Error("create application failed", zap.Error(err))
		return uuid.Nil, err
	}

	log.Info("application created", zap.String("application_id", app.GetID().String()))
	return app.GetID(), nil
}

func (s *postgresStore) Get(ctx context.Context, id uuid.UUID) (*appdomain.Application, error) {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "get_application"),
		zap.String("application_id", id.String()),
	)

	app, err := scanApplication(platformdb.Postgres().QueryRowContext(ctx, `
		select id, project_id, name, repo_address, description, active_image_id, labels, created_at, updated_at, deleted_at
		from applications
		where id = $1 and deleted_at is null
	`, id))
	if err != nil {
		log.Error("get application failed", zap.Error(err))
		return nil, err
	}

	log.Debug("application fetched", zap.String("application_name", app.Name))
	return app, nil
}

func (s *postgresStore) Update(ctx context.Context, app *appdomain.Application) error {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "update_application"),
		zap.String("application_id", app.GetID().String()),
	)

	current, err := s.Get(ctx, app.GetID())
	if err != nil {
		log.Error("load application failed", zap.Error(err))
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
		set project_id=$2, name=$3, repo_address=$4, description=$5, active_image_id=$6, labels=$7, updated_at=$8, deleted_at=$9
		where id = $1 and deleted_at is null
	`, app.ID, nullableUUID(app.ProjectID), app.Name, app.RepoAddress, app.Description, nullableUUIDPtr(app.ActiveImageID), labels, app.UpdatedAt, app.DeletedAt)
	if err != nil {
		log.Error("update application failed", zap.Error(err))
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	log.Debug("application updated", zap.String("application_name", app.Name))
	return nil
}

func (s *postgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "delete_application"),
		zap.String("application_id", id.String()),
	)

	now := time.Now()
	result, err := platformdb.Postgres().ExecContext(ctx, `
		update applications
		set deleted_at=$2, updated_at=$2
		where id = $1 and deleted_at is null
	`, id, now)
	if err != nil {
		log.Error("delete application failed", zap.Error(err))
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	log.Info("application deleted")
	return nil
}

func (s *postgresStore) UpdateActiveImage(ctx context.Context, appID, imageID uuid.UUID) error {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "update_application_active_image"),
		zap.String("application_id", appID.String()),
		zap.String("image_id", imageID.String()),
	)

	result, err := platformdb.Postgres().ExecContext(ctx, `
		update applications
		set active_image_id=$2, updated_at=$3
		where id = $1 and deleted_at is null
	`, appID, imageID, time.Now())
	if err != nil {
		log.Error("update active image failed", zap.Error(err))
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *postgresStore) List(ctx context.Context, includeDeleted bool, name string, projectID *uuid.UUID, repoAddress string) ([]appdomain.Application, error) {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "list_applications"),
		zap.Bool("include_deleted", includeDeleted),
		zap.String("name", name),
		zap.String("repo_address", repoAddress),
	)

	query := `
		select id, project_id, name, repo_address, description, active_image_id, labels, created_at, updated_at, deleted_at
		from applications
	`
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)

	if !includeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if name != "" {
		args = append(args, name)
		clauses = append(clauses, placeholderClause("name", len(args)))
	}
	if projectID != nil {
		args = append(args, *projectID)
		clauses = append(clauses, placeholderClause("project_id", len(args)))
	}
	if repoAddress != "" {
		args = append(args, repoAddress)
		clauses = append(clauses, placeholderClause("repo_address", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"

	rows, err := platformdb.Postgres().QueryContext(ctx, query, args...)
	if err != nil {
		log.Error("list applications failed", zap.Error(err))
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

	log.Debug("applications listed", zap.Int("count", len(apps)))
	return apps, nil
}

func scanApplication(scanner interface {
	Scan(dest ...any) error
}) (*appdomain.Application, error) {
	var (
		app           appdomain.Application
		projectID     sql.NullString
		activeImageID sql.NullString
		labelsBytes   []byte
		deletedAt     sql.NullTime
	)

	if err := scanner.Scan(
		&app.ID,
		&projectID,
		&app.Name,
		&app.RepoAddress,
		&app.Description,
		&activeImageID,
		&labelsBytes,
		&app.CreatedAt,
		&app.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}

	if projectID.Valid {
		parsed, err := uuid.Parse(projectID.String)
		if err != nil {
			return nil, err
		}
		app.ProjectID = parsed
	}
	if activeImageID.Valid {
		parsed, err := uuid.Parse(activeImageID.String)
		if err != nil {
			return nil, err
		}
		app.ActiveImageID = &parsed
	}
	if deletedAt.Valid {
		app.DeletedAt = &deletedAt.Time
	}
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
	if labels == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(labels)
}

func unmarshalLabels(raw []byte) ([]appdomain.LabelItem, error) {
	var labels []appdomain.LabelItem
	if err := json.Unmarshal(raw, &labels); err == nil {
		return labels, nil
	}
	var legacy map[string]string
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, err
	}
	labels = make([]appdomain.LabelItem, 0, len(legacy))
	for key, value := range legacy {
		labels = append(labels, appdomain.LabelItem{Key: key, Value: value})
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i].Key < labels[j].Key })
	return labels, nil
}

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func nullableUUIDPtr(id *uuid.UUID) any {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	return *id
}

func placeholderClause(column string, position int) string {
	return column + " = $" + strconv.Itoa(position)
}
