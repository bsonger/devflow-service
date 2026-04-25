package support

import (
	"context"
	"database/sql"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	store "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ApplicationProjection struct {
	ID          uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
	Name        string
	ProjectName string
	RepoAddress string
	RepoURL     string
}

var ApplicationService = NewApplicationService()

type applicationService struct{}

func NewApplicationService() *applicationService {
	return &applicationService{}
}

func (s *applicationService) Get(ctx context.Context, id uuid.UUID) (*ApplicationProjection, error) {
	log := logger.LoggerWithContext(ctx).With(
		zap.String("operation", "get_application"),
		zap.String("application_id", id.String()),
	)

	row := store.DB().QueryRowContext(ctx, `
		select
			a.id,
			a.name,
			coalesce(p.name, ''),
			a.repo_address,
			a.created_at,
			a.updated_at,
			a.deleted_at
		from applications a
		left join projects p on p.id = a.project_id and p.deleted_at is null
		where a.id = $1 and a.deleted_at is null
	`, id)

	app, err := scanApplicationProjection(row)
	if err != nil {
		log.Error("get application failed", zap.Error(err))
		return nil, err
	}

	log.Debug("application fetched", zap.String("application_name", app.Name))
	return app, nil
}

func scanApplicationProjection(scanner interface {
	Scan(dest ...any) error
}) (*ApplicationProjection, error) {
	var (
		app       ApplicationProjection
		deletedAt sql.NullTime
	)

	if err := scanner.Scan(
		&app.ID,
		&app.Name,
		&app.ProjectName,
		&app.RepoAddress,
		&app.CreatedAt,
		&app.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}

	if deletedAt.Valid {
		app.DeletedAt = &deletedAt.Time
	}

	return &app, nil
}
