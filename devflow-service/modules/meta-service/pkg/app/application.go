package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	loggingx "github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/domain"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/infra/store"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var ApplicationService = NewApplicationService()

var ErrProjectReferenceNotFound = errors.New("project reference not found")

type ApplicationListFilter struct {
	IncludeDeleted bool
	Name           string
	ProjectID      *uuid.UUID
	RepoAddress    string
}

type applicationService struct{}

func NewApplicationService() *applicationService {
	return &applicationService{}
}

func (s *applicationService) Create(ctx context.Context, app *domain.Application) (uuid.UUID, error) {
	log := loggingx.LoggerWithContext(ctx).With(zap.String("operation", "create_application"))

	if err := s.syncProjectReference(ctx, app); err != nil {
		log.Error("resolve project reference failed", zap.Error(err))
		return uuid.Nil, err
	}
	labels, err := marshalLabels(app.Labels)
	if err != nil {
		log.Error("marshal application labels failed", zap.Error(err))
		return uuid.Nil, err
	}

	_, err = store.DB().ExecContext(ctx, `
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

func (s *applicationService) Get(ctx context.Context, id uuid.UUID) (*domain.Application, error) {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "get_application"),
		zap.String("application_id", id.String()),
	)

	app, err := scanApplication(store.DB().QueryRowContext(ctx, `
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

func (s *applicationService) Update(ctx context.Context, app *domain.Application) error {
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

	if err := s.syncProjectReference(ctx, app); err != nil {
		log.Error("resolve project reference failed", zap.Error(err))
		return err
	}
	labels, err := marshalLabels(app.Labels)
	if err != nil {
		return err
	}

	result, err := store.DB().ExecContext(ctx, `
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

func (s *applicationService) Delete(ctx context.Context, id uuid.UUID) error {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "delete_application"),
		zap.String("application_id", id.String()),
	)

	now := time.Now()
	result, err := store.DB().ExecContext(ctx, `
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

func (s *applicationService) UpdateActiveImage(ctx context.Context, appID, imageID uuid.UUID) error {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "update_application_active_image"),
		zap.String("application_id", appID.String()),
		zap.String("image_id", imageID.String()),
	)

	result, err := store.DB().ExecContext(ctx, `
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

func (s *applicationService) List(ctx context.Context, filter ApplicationListFilter) ([]domain.Application, error) {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "list_applications"),
		zap.Any("filter", filter),
	)

	query := `
		select id, project_id, name, repo_address, description, active_image_id, labels, created_at, updated_at, deleted_at
		from applications
	`
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)

	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.Name != "" {
		args = append(args, filter.Name)
		clauses = append(clauses, placeholderClause("name", len(args)))
	}
	if filter.ProjectID != nil {
		args = append(args, *filter.ProjectID)
		clauses = append(clauses, placeholderClause("project_id", len(args)))
	}
	if filter.RepoAddress != "" {
		args = append(args, filter.RepoAddress)
		clauses = append(clauses, placeholderClause("repo_address", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"

	rows, err := store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		log.Error("list applications failed", zap.Error(err))
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	apps := make([]domain.Application, 0)
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

func (s *applicationService) syncProjectReference(ctx context.Context, app *domain.Application) error {
	if app.ProjectID == uuid.Nil {
		return nil
	}

	if _, err := ProjectService.Get(ctx, app.ProjectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProjectReferenceNotFound
		}
		return err
	}
	return nil
}

func scanApplication(scanner interface {
	Scan(dest ...any) error
}) (*domain.Application, error) {
	var (
		app           domain.Application
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
