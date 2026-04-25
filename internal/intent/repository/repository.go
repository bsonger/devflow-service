package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	intentdomain "github.com/bsonger/devflow-service/internal/intent/domain"
	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

type ListFilter struct {
	Kind         string
	Status       string
	ResourceType string
	ClaimedBy    string
	ResourceID   *uuid.UUID
}

type Store interface {
	Insert(ctx context.Context, intent *intentdomain.Intent) error
	Get(ctx context.Context, id uuid.UUID) (*intentdomain.Intent, error)
	List(ctx context.Context, filter ListFilter) ([]*intentdomain.Intent, error)
	ListPending(ctx context.Context, limit int) ([]intentdomain.Intent, error)
	ClaimNextPending(ctx context.Context, workerID string, leaseDuration time.Duration) (*intentdomain.Intent, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.IntentStatus, message string) error
	MarkSubmitted(ctx context.Context, id uuid.UUID, message string) error
	MarkFailed(ctx context.Context, id uuid.UUID, message string) error
	UpdateStatusByResource(ctx context.Context, kind model.IntentKind, resourceID uuid.UUID, status model.IntentStatus, message string) error
	BindIntentToImage(ctx context.Context, imageID, intentID uuid.UUID) error
	BindIntentToRelease(ctx context.Context, releaseID, intentID uuid.UUID) error
}

type PostgresStore struct{}

func NewPostgresStore() Store {
	return &PostgresStore{}
}

func (s *PostgresStore) Insert(ctx context.Context, intent *intentdomain.Intent) error {
	_, err := db.DB().ExecContext(ctx, `
		insert into execution_intents (
			id, kind, status, resource_type, resource_id, trace_id, message, last_error, claimed_by, claimed_at, lease_expires_at, attempt_count, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, intent.ID, intent.Kind, intent.Status, intent.ResourceType, intent.ResourceID, intent.TraceID, intent.Message, intent.LastError, intent.ClaimedBy, dbsql.NullableTimePtr(intent.ClaimedAt), dbsql.NullableTimePtr(intent.LeaseExpiresAt), intent.AttemptCount, intent.CreatedAt, intent.UpdatedAt, intent.DeletedAt)
	return err
}

func (s *PostgresStore) Get(ctx context.Context, id uuid.UUID) (*intentdomain.Intent, error) {
	return scanIntent(db.DB().QueryRowContext(ctx, `
		select id, kind, status, resource_type, resource_id, trace_id, message, last_error, claimed_by, claimed_at, lease_expires_at, attempt_count, created_at, updated_at, deleted_at
		from execution_intents
		where id = $1 and deleted_at is null
	`, id))
}

func (s *PostgresStore) List(ctx context.Context, filter ListFilter) ([]*intentdomain.Intent, error) {
	query := `
		select id, kind, status, resource_type, resource_id, trace_id, message, last_error, claimed_by, claimed_at, lease_expires_at, attempt_count, created_at, updated_at, deleted_at
		from execution_intents
	`
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	if filter.Kind != "" {
		args = append(args, filter.Kind)
		clauses = append(clauses, dbsql.PlaceholderClause("kind", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, dbsql.PlaceholderClause("status", len(args)))
	}
	if filter.ResourceType != "" {
		args = append(args, filter.ResourceType)
		clauses = append(clauses, dbsql.PlaceholderClause("resource_type", len(args)))
	}
	if filter.ClaimedBy != "" {
		args = append(args, filter.ClaimedBy)
		clauses = append(clauses, dbsql.PlaceholderClause("claimed_by", len(args)))
	}
	if filter.ResourceID != nil {
		args = append(args, *filter.ResourceID)
		clauses = append(clauses, dbsql.PlaceholderClause("resource_id", len(args)))
	}
	clauses = append(clauses, "deleted_at is null")
	query += " where " + strings.Join(clauses, " and ") + " order by created_at desc"

	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*intentdomain.Intent, 0)
	for rows.Next() {
		intent, err := scanIntent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, intent)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) ListPending(ctx context.Context, limit int) ([]intentdomain.Intent, error) {
	query := `
		select id, kind, status, resource_type, resource_id, trace_id, message, last_error, claimed_by, claimed_at, lease_expires_at, attempt_count, created_at, updated_at, deleted_at
		from execution_intents
		where status = $1 and deleted_at is null
		order by created_at asc
	`
	args := []any{model.IntentPending}
	if limit > 0 {
		query += ` limit $2`
		args = append(args, limit)
	}
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]intentdomain.Intent, 0)
	for rows.Next() {
		intent, err := scanIntent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *intent)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) ClaimNextPending(ctx context.Context, workerID string, leaseDuration time.Duration) (*intentdomain.Intent, error) {
	now := time.Now()
	leaseExpiresAt := now.Add(leaseDuration)
	return scanIntent(db.DB().QueryRowContext(ctx, `
		update execution_intents
		set claimed_by = $1, claimed_at = $2, lease_expires_at = $3, updated_at = $2, message = 'claimed by worker', attempt_count = attempt_count + 1
		where id = (
			select id
			from execution_intents
			where status = $4
			  and deleted_at is null
			  and (claimed_by = '' or lease_expires_at is null or lease_expires_at < $2)
			order by created_at asc
			limit 1
			for update skip locked
		)
		returning id, kind, status, resource_type, resource_id, trace_id, message, last_error, claimed_by, claimed_at, lease_expires_at, attempt_count, created_at, updated_at, deleted_at
	`, workerID, now, leaseExpiresAt, model.IntentPending))
}

func (s *PostgresStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.IntentStatus, message string) error {
	result, err := db.DB().ExecContext(ctx, `
		update execution_intents
		set status=$2, message=$3, last_error='', updated_at=$4
		where id = $1 and deleted_at is null
	`, id, status, message, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) MarkSubmitted(ctx context.Context, id uuid.UUID, message string) error {
	result, err := db.DB().ExecContext(ctx, `
		update execution_intents
		set status=$2, message=$3, last_error='', updated_at=$4, claimed_by='', claimed_at=null, lease_expires_at=null
		where id = $1 and deleted_at is null
	`, id, model.IntentRunning, message, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) MarkFailed(ctx context.Context, id uuid.UUID, message string) error {
	result, err := db.DB().ExecContext(ctx, `
		update execution_intents
		set status=$2, message=$3, last_error=$3, updated_at=$4, claimed_by='', claimed_at=null, lease_expires_at=null
		where id = $1 and deleted_at is null
	`, id, model.IntentFailed, message, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) UpdateStatusByResource(ctx context.Context, kind model.IntentKind, resourceID uuid.UUID, status model.IntentStatus, message string) error {
	result, err := db.DB().ExecContext(ctx, `
		update execution_intents
		set status=$3, message=$4, updated_at=$5
		where kind = $1 and resource_id = $2 and deleted_at is null
	`, kind, resourceID, status, message, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) BindIntentToImage(ctx context.Context, imageID, intentID uuid.UUID) error {
	result, err := db.DB().ExecContext(ctx, `
		update images
		set execution_intent_id = $2, updated_at = $3
		where id = $1 and deleted_at is null
	`, imageID, intentID, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) BindIntentToRelease(ctx context.Context, releaseID, intentID uuid.UUID) error {
	result, err := db.DB().ExecContext(ctx, `
		update releases
		set execution_intent_id = $2, updated_at = $3
		where id = $1 and deleted_at is null
	`, releaseID, intentID, time.Now())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func scanIntent(scanner interface{ Scan(dest ...any) error }) (*intentdomain.Intent, error) {
	var (
		item           intentdomain.Intent
		claimedAt      sql.NullTime
		leaseExpiresAt sql.NullTime
		deletedAt      sql.NullTime
	)

	if err := scanner.Scan(
		&item.ID,
		&item.Kind,
		&item.Status,
		&item.ResourceType,
		&item.ResourceID,
		&item.TraceID,
		&item.Message,
		&item.LastError,
		&item.ClaimedBy,
		&claimedAt,
		&leaseExpiresAt,
		&item.AttemptCount,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	item.ClaimedAt = dbsql.TimePtrFromNull(claimedAt)
	item.LeaseExpiresAt = dbsql.TimePtrFromNull(leaseExpiresAt)
	item.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	return &item, nil
}
