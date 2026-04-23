package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/modules/meta-service/pkg/domain"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/infra/store"
	"github.com/bsonger/devflow-service/shared/loggingx"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var ProjectService = NewProjectService()

type ProjectListFilter struct {
	IncludeDeleted bool
	Name           string
}

type projectService struct{}

func NewProjectService() *projectService {
	return &projectService{}
}

func (s *projectService) Create(ctx context.Context, project *domain.Project) (uuid.UUID, error) {
	log := loggingx.LoggerWithContext(ctx).With(zap.String("operation", "create_project"))

	labels, err := marshalLabels(project.Labels)
	if err != nil {
		log.Error("marshal project labels failed", zap.Error(err))
		return uuid.Nil, err
	}

	_, err = store.DB().ExecContext(ctx, `
		insert into projects (
			id, name, description, labels, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7)
	`, project.ID, project.Name, project.Description, labels, project.CreatedAt, project.UpdatedAt, project.DeletedAt)
	if err != nil {
		log.Error("create project failed", zap.Error(err))
		return uuid.Nil, err
	}

	log.Info("project created", zap.String("project_id", project.GetID().String()), zap.String("project_name", project.Name))
	return project.GetID(), nil
}

func (s *projectService) Get(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "get_project"),
		zap.String("project_id", id.String()),
	)

	project, err := scanProject(store.DB().QueryRowContext(ctx, `
		select id, name, description, labels, created_at, updated_at, deleted_at
		from projects
		where id = $1 and deleted_at is null
	`, id))
	if err != nil {
		log.Error("get project failed", zap.Error(err))
		return nil, err
	}

	log.Debug("project fetched", zap.String("project_name", project.Name))
	return project, nil
}

func (s *projectService) Update(ctx context.Context, project *domain.Project) error {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "update_project"),
		zap.String("project_id", project.GetID().String()),
	)

	current, err := s.Get(ctx, project.GetID())
	if err != nil {
		log.Error("load project failed", zap.Error(err))
		return err
	}

	project.CreatedAt = current.CreatedAt
	project.DeletedAt = current.DeletedAt
	project.WithUpdateDefault()

	labels, err := marshalLabels(project.Labels)
	if err != nil {
		return err
	}

	result, err := store.DB().ExecContext(ctx, `
		update projects
		set name=$2, description=$3, labels=$4, updated_at=$5, deleted_at=$6
		where id = $1 and deleted_at is null
	`, project.ID, project.Name, project.Description, labels, project.UpdatedAt, project.DeletedAt)
	if err != nil {
		log.Error("update project failed", zap.Error(err))
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	log.Info("project updated", zap.String("project_name", project.Name))
	return nil
}

func (s *projectService) Delete(ctx context.Context, id uuid.UUID) error {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "delete_project"),
		zap.String("project_id", id.String()),
	)

	now := time.Now()
	result, err := store.DB().ExecContext(ctx, `
		update projects
		set deleted_at=$2, updated_at=$2
		where id = $1 and deleted_at is null
	`, id, now)
	if err != nil {
		log.Error("delete project failed", zap.Error(err))
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	log.Info("project deleted")
	return nil
}

func (s *projectService) List(ctx context.Context, filter ProjectListFilter) ([]domain.Project, error) {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "list_projects"),
		zap.Any("filter", filter),
	)

	query := `
		select id, name, description, labels, created_at, updated_at, deleted_at
		from projects
	`
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)

	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.Name != "" {
		args = append(args, filter.Name)
		clauses = append(clauses, placeholderClause("name", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"

	rows, err := store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		log.Error("list projects failed", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	projects := make([]domain.Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *project)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	log.Debug("projects listed", zap.Int("count", len(projects)))
	return projects, nil
}

func (s *projectService) ListApplications(ctx context.Context, projectID uuid.UUID) ([]domain.Application, error) {
	if _, err := s.Get(ctx, projectID); err != nil {
		return nil, err
	}

	return ApplicationService.List(ctx, ApplicationListFilter{ProjectID: &projectID})
}

func scanProject(scanner interface {
	Scan(dest ...any) error
}) (*domain.Project, error) {
	var (
		project     domain.Project
		labelsBytes []byte
		deletedAt   sql.NullTime
	)

	if err := scanner.Scan(
		&project.ID,
		&project.Name,
		&project.Description,
		&labelsBytes,
		&project.CreatedAt,
		&project.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}

	if deletedAt.Valid {
		project.DeletedAt = &deletedAt.Time
	}
	if len(labelsBytes) > 0 {
		labels, err := unmarshalLabels(labelsBytes)
		if err != nil {
			return nil, err
		}
		project.Labels = labels
	}

	return &project, nil
}

func marshalLabels(labels []domain.LabelItem) ([]byte, error) {
	if labels == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(labels)
}

func unmarshalLabels(raw []byte) ([]domain.LabelItem, error) {
	var labels []domain.LabelItem
	if err := json.Unmarshal(raw, &labels); err == nil {
		return labels, nil
	}
	var legacy map[string]string
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, err
	}
	labels = make([]domain.LabelItem, 0, len(legacy))
	for key, value := range legacy {
		labels = append(labels, domain.LabelItem{Key: key, Value: value})
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i].Key < labels[j].Key })
	return labels, nil
}

func placeholderClause(column string, position int) string {
	return column + " = $" + strconv.Itoa(position)
}
