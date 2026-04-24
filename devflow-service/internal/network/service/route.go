package service

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/network/domain"
	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/google/uuid"
)

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func ensureRowsAffected(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func placeholderClause(column string, position int) string {
	return column + " = $" + strconv.Itoa(position)
}

type RouteService interface {
	Create(ctx context.Context, route *domain.Route) (uuid.UUID, error)
	Get(ctx context.Context, applicationID, id uuid.UUID) (*domain.Route, error)
	Update(ctx context.Context, route *domain.Route) error
	Delete(ctx context.Context, applicationID, id uuid.UUID) error
	List(ctx context.Context, filter RouteListFilter) ([]domain.Route, error)
	Validate(ctx context.Context, route *domain.Route) []string
}

type RouteListFilter struct {
	ApplicationID  uuid.UUID
	IncludeDeleted bool
	Name           string
}

type routeService struct {
	services ServiceService
}

func NewRouteService(services ServiceService) RouteService {
	return &routeService{services: services}
}

func (s *routeService) Create(ctx context.Context, item *domain.Route) (uuid.UUID, error) {
	if err := s.validate(ctx, item); err != nil {
		return uuid.Nil, err
	}
	_, err := db.Postgres().ExecContext(ctx, `
		insert into routes (
			id, application_id, name, host, path, service_name, service_port, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, item.ID, nullableUUID(item.ApplicationID), item.Name, item.Host, item.Path, item.ServiceName, item.ServicePort, item.CreatedAt, item.UpdatedAt, item.DeletedAt)
	if err != nil {
		return uuid.Nil, err
	}
	return item.ID, nil
}

func (s *routeService) Get(ctx context.Context, applicationID, id uuid.UUID) (*domain.Route, error) {
	return scanRoute(db.Postgres().QueryRowContext(ctx, `
		select id, application_id, name, host, path, service_name, service_port, created_at, updated_at, deleted_at
		from routes
		where application_id = $1 and id = $2 and deleted_at is null
	`, applicationID, id))
}

func (s *routeService) Update(ctx context.Context, item *domain.Route) error {
	if err := s.validate(ctx, item); err != nil {
		return err
	}
	current, err := s.Get(ctx, item.ApplicationID, item.ID)
	if err != nil {
		return err
	}
	item.CreatedAt = current.CreatedAt
	item.DeletedAt = current.DeletedAt
	item.WithUpdateDefault()
	result, err := db.Postgres().ExecContext(ctx, `
		update routes
		set name=$3, host=$4, path=$5, service_name=$6, service_port=$7, updated_at=$8
		where application_id=$1 and id=$2 and deleted_at is null
	`, item.ApplicationID, item.ID, item.Name, item.Host, item.Path, item.ServiceName, item.ServicePort, item.UpdatedAt)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *routeService) Delete(ctx context.Context, applicationID, id uuid.UUID) error {
	now := time.Now()
	result, err := db.Postgres().ExecContext(ctx, `
		update routes set deleted_at=$3, updated_at=$3
		where application_id=$1 and id=$2 and deleted_at is null
	`, applicationID, id, now)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *routeService) List(ctx context.Context, filter RouteListFilter) ([]domain.Route, error) {
	query := `
		select id, application_id, name, host, path, service_name, service_port, created_at, updated_at, deleted_at
		from routes
	`
	clauses := []string{"application_id = $1"}
	args := []any{filter.ApplicationID}
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.Name != "" {
		args = append(args, filter.Name)
		clauses = append(clauses, placeholderClause("name", len(args)))
	}
	query += " where " + strings.Join(clauses, " and ") + " order by created_at desc"
	rows, err := db.Postgres().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.Route
	for rows.Next() {
		item, err := scanRoute(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *routeService) Validate(ctx context.Context, item *domain.Route) []string {
	var errs []string
	if item == nil {
		return []string{"route is required"}
	}
	if item.ApplicationID == uuid.Nil {
		errs = append(errs, "application_id is required")
	}
	if strings.TrimSpace(item.Name) == "" {
		errs = append(errs, "name is required")
	}
	if strings.TrimSpace(item.Host) == "" {
		errs = append(errs, "host is required")
	}
	if strings.TrimSpace(item.Path) == "" {
		errs = append(errs, "path is required")
	}
	if strings.TrimSpace(item.ServiceName) == "" {
		errs = append(errs, "service_name is required")
	}
	if item.ServicePort <= 0 {
		errs = append(errs, "service_port is required")
	}
	if len(errs) > 0 {
		return errs
	}
	services, err := s.services.List(ctx, ServiceListFilter{
		ApplicationID: item.ApplicationID,
		Name:          item.ServiceName,
	})
	if err != nil {
		return []string{err.Error()}
	}
	if len(services) == 0 {
		return []string{"service_name does not exist"}
	}
	for _, port := range services[0].Ports {
		if port.ServicePort == item.ServicePort {
			return nil
		}
	}
	return []string{"service_port does not exist on target service"}
}

func (s *routeService) validate(ctx context.Context, item *domain.Route) error {
	if errs := s.Validate(ctx, item); len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func scanRoute(scanner interface{ Scan(dest ...any) error }) (*domain.Route, error) {
	var (
		item          domain.Route
		applicationID sql.NullString
		deletedAt     sql.NullTime
	)
	if err := scanner.Scan(&item.ID, &applicationID, &item.Name, &item.Host, &item.Path, &item.ServiceName, &item.ServicePort, &item.CreatedAt, &item.UpdatedAt, &deletedAt); err != nil {
		return nil, err
	}
	if applicationID.Valid {
		parsed, err := uuid.Parse(applicationID.String)
		if err != nil {
			return nil, err
		}
		item.ApplicationID = parsed
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
	return &item, nil
}
