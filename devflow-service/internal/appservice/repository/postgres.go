package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/appservice/domain"
	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
)

type Store interface {
	Create(ctx context.Context, network *domain.Network) (uuid.UUID, error)
	Get(ctx context.Context, applicationID, id uuid.UUID) (*domain.Network, error)
	Update(ctx context.Context, network *domain.Network) error
	Delete(ctx context.Context, applicationID, id uuid.UUID) error
	List(ctx context.Context, filter NetworkListFilter) ([]domain.Network, error)
}

type NetworkListFilter struct {
	ApplicationID  uuid.UUID
	IncludeDeleted bool
	Name           string
}

type postgresStore struct{}

func NewPostgresStore() Store {
	return &postgresStore{}
}

var NetworkStore Store = NewPostgresStore()

func (s *postgresStore) Create(ctx context.Context, network *domain.Network) (uuid.UUID, error) {
	if err := validateNetwork(network); err != nil {
		return uuid.Nil, err
	}

	portsJSON, _ := json.Marshal(network.Ports)
	hostsJSON, _ := json.Marshal(network.Hosts)
	pathsJSON, _ := json.Marshal(network.Paths)
	gatewaysJSON, _ := json.Marshal(network.GatewayRefs)

	_, err := db.Postgres().ExecContext(ctx, `
		insert into networks (
			id, application_id, name, ports, hosts, paths, gateway_refs, visibility, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, network.ID, dbsql.NullableUUID(network.ApplicationID), network.Name, portsJSON, hostsJSON, pathsJSON, gatewaysJSON, network.Visibility, network.CreatedAt, network.UpdatedAt, network.DeletedAt)
	if err != nil {
		return uuid.Nil, err
	}
	return network.ID, nil
}

func (s *postgresStore) Get(ctx context.Context, applicationID, id uuid.UUID) (*domain.Network, error) {
	return scanNetwork(db.Postgres().QueryRowContext(ctx, `
		select id, application_id, name, ports, hosts, paths, gateway_refs, visibility, created_at, updated_at, deleted_at
		from networks
		where application_id = $1 and id = $2 and deleted_at is null
	`, applicationID, id))
}

func (s *postgresStore) Update(ctx context.Context, network *domain.Network) error {
	if err := validateNetwork(network); err != nil {
		return err
	}

	current, err := s.Get(ctx, network.ApplicationID, network.ID)
	if err != nil {
		return err
	}
	network.CreatedAt = current.CreatedAt
	network.DeletedAt = current.DeletedAt
	network.WithUpdateDefault()

	portsJSON, _ := json.Marshal(network.Ports)
	hostsJSON, _ := json.Marshal(network.Hosts)
	pathsJSON, _ := json.Marshal(network.Paths)
	gatewaysJSON, _ := json.Marshal(network.GatewayRefs)

	result, err := db.Postgres().ExecContext(ctx, `
		update networks
		set name=$3, ports=$4, hosts=$5, paths=$6, gateway_refs=$7, visibility=$8, updated_at=$9
		where application_id=$1 and id=$2 and deleted_at is null
	`, network.ApplicationID, network.ID, network.Name, portsJSON, hostsJSON, pathsJSON, gatewaysJSON, network.Visibility, network.UpdatedAt)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *postgresStore) Delete(ctx context.Context, applicationID, id uuid.UUID) error {
	now := time.Now()
	result, err := db.Postgres().ExecContext(ctx, `
		update networks set deleted_at=$3, updated_at=$3
		where application_id=$1 and id=$2 and deleted_at is null
	`, applicationID, id, now)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *postgresStore) List(ctx context.Context, filter NetworkListFilter) ([]domain.Network, error) {
	query := `
		select id, application_id, name, ports, hosts, paths, gateway_refs, visibility, created_at, updated_at, deleted_at
		from networks
	`
	clauses := []string{"application_id = $1"}
	args := []any{filter.ApplicationID}
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.Name != "" {
		args = append(args, filter.Name)
		clauses = append(clauses, dbsql.PlaceholderClause("name", len(args)))
	}
	query += " where " + strings.Join(clauses, " and ") + " order by created_at desc"

	rows, err := db.Postgres().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Network
	for rows.Next() {
		item, err := scanNetwork(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func validateNetwork(network *domain.Network) error {
	if network == nil {
		return sharederrs.Required("network")
	}
	if network.ApplicationID == uuid.Nil {
		return sharederrs.Required("application_id")
	}
	if strings.TrimSpace(network.Name) == "" {
		return sharederrs.Required("name")
	}
	return nil
}

func scanNetwork(scanner interface{ Scan(dest ...any) error }) (*domain.Network, error) {
	var (
		item          domain.Network
		applicationID sql.NullString
		portsJSON     []byte
		hostsJSON     []byte
		pathsJSON     []byte
		gatewaysJSON  []byte
		deletedAt     sql.NullTime
	)
	if err := scanner.Scan(&item.ID, &applicationID, &item.Name, &portsJSON, &hostsJSON, &pathsJSON, &gatewaysJSON, &item.Visibility, &item.CreatedAt, &item.UpdatedAt, &deletedAt); err != nil {
		return nil, err
	}
	applicationUUID, err := dbsql.ParseNullUUID(applicationID)
	if err != nil {
		return nil, err
	}
	if applicationUUID != nil {
		item.ApplicationID = *applicationUUID
	}
	if len(portsJSON) > 0 {
		if err := json.Unmarshal(portsJSON, &item.Ports); err != nil {
			return nil, err
		}
	}
	if len(hostsJSON) > 0 {
		if err := json.Unmarshal(hostsJSON, &item.Hosts); err != nil {
			return nil, err
		}
	}
	if len(pathsJSON) > 0 {
		if err := json.Unmarshal(pathsJSON, &item.Paths); err != nil {
			return nil, err
		}
	}
	if len(gatewaysJSON) > 0 {
		if err := json.Unmarshal(gatewaysJSON, &item.GatewayRefs); err != nil {
			return nil, err
		}
	}
	item.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	return &item, nil
}
