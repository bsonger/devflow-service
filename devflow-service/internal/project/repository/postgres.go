package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	platformdb "github.com/bsonger/devflow-service/internal/platform/db"
	loggingx "github.com/bsonger/devflow-service/internal/platform/logger"
	projectdomain "github.com/bsonger/devflow-service/internal/project/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Store interface {
	Create(context.Context, *projectdomain.Project) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*projectdomain.Project, error)
	Update(context.Context, *projectdomain.Project) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, bool, string) ([]projectdomain.Project, error)
}

var ProjectStore Store = NewPostgresStore()

type postgresStore struct{}

func NewPostgresStore() Store {
	return &postgresStore{}
}

func (s *postgresStore) Create(ctx context.Context, project *projectdomain.Project) (uuid.UUID, error) {
	log := loggingx.LoggerWithContext(ctx).With(zap.String("operation", "create_project"))

	labels, err := marshalLabels(project.Labels)
	if err != nil {
		log.Error("marshal project labels failed", zap.Error(err))
		return uuid.Nil, err
	}

	_, err = platformdb.Postgres().ExecContext(ctx, `
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

func (s *postgresStore) Get(ctx context.Context, id uuid.UUID) (*projectdomain.Project, error) {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "get_project"),
		zap.String("project_id", id.String()),
	)

	project, err := scanProject(platformdb.Postgres().QueryRowContext(ctx, `
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

func (s *postgresStore) Update(ctx context.Context, project *projectdomain.Project) error {
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

	result, err := platformdb.Postgres().ExecContext(ctx, `
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

func (s *postgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "delete_project"),
		zap.String("project_id", id.String()),
	)

	now := time.Now()
	result, err := platformdb.Postgres().ExecContext(ctx, `
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

func (s *postgresStore) List(ctx context.Context, includeDeleted bool, name string) ([]projectdomain.Project, error) {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "list_projects"),
		zap.Bool("include_deleted", includeDeleted),
		zap.String("name", name),
	)

	query := `
		select id, name, description, labels, created_at, updated_at, deleted_at
		from projects
	`
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)

	if !includeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if name != "" {
		args = append(args, name)
		clauses = append(clauses, placeholderClause("name", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"

	rows, err := platformdb.Postgres().QueryContext(ctx, query, args...)
	if err != nil {
		log.Error("list projects failed", zap.Error(err))
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	projects := make([]projectdomain.Project, 0)
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

func scanProject(scanner interface {
	Scan(dest ...any) error
}) (*projectdomain.Project, error) {
	var (
		project     projectdomain.Project
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

func marshalLabels(labels []projectdomain.LabelItem) ([]byte, error) {
	if labels == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(labels)
}

func unmarshalLabels(raw []byte) ([]projectdomain.LabelItem, error) {
	var labels []projectdomain.LabelItem
	if err := json.Unmarshal(raw, &labels); err == nil {
		return labels, nil
	}
	var legacy map[string]string
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, err
	}
	labels = make([]projectdomain.LabelItem, 0, len(legacy))
	for key, value := range legacy {
		labels = append(labels, projectdomain.LabelItem{Key: key, Value: value})
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i].Key < labels[j].Key })
	return labels, nil
}

func placeholderClause(column string, position int) string {
	return column + " = $" + strconv.Itoa(position)
}
