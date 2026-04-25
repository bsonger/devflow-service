package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	intentdomain "github.com/bsonger/devflow-service/internal/intent/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	store "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type IntentListFilter struct {
	Kind         string
	Status       string
	ResourceType string
	ClaimedBy    string
	ResourceID   *uuid.UUID
}

var IntentService = &intentService{}

type intentService struct{}

var ErrIntentNotFound = errors.New("intent not found")

func (s *intentService) CreateBuildIntent(ctx context.Context, image *imagedomain.Image) (uuid.UUID, error) {
	intent := &intentdomain.Intent{
		Kind:         model.IntentKindBuild,
		Status:       model.IntentPending,
		ResourceType: "image",
		ResourceID:   image.ID,
	}
	intent.WithCreateDefault()
	if err := s.insert(ctx, intent); err != nil {
		return uuid.Nil, err
	}
	if err := s.bindIntentToImage(ctx, image.ID, intent.ID); err != nil {
		return intent.ID, err
	}
	image.ExecutionIntentID = uuidPtr(intent.ID)
	logger.LoggerWithContext(ctx).Info("build intent created", zap.String("intent_id", intent.ID.String()), zap.String("image_id", image.ID.String()))
	return intent.ID, nil
}

func (s *intentService) CreateReleaseIntent(ctx context.Context, release *model.Release) (uuid.UUID, error) {
	intent := &intentdomain.Intent{
		Kind:         model.IntentKindRelease,
		Status:       model.IntentPending,
		ResourceType: "release",
		ResourceID:   release.ID,
	}
	intent.WithCreateDefault()
	if err := s.insert(ctx, intent); err != nil {
		return uuid.Nil, err
	}
	if err := s.bindIntentToRelease(ctx, release.ID, intent.ID); err != nil {
		return intent.ID, err
	}
	release.ExecutionIntentID = uuidPtr(intent.ID)
	logger.LoggerWithContext(ctx).Info("release intent created", zap.String("intent_id", intent.ID.String()), zap.String("release_id", release.ID.String()))
	return intent.ID, nil
}

func (s *intentService) insert(ctx context.Context, intent *intentdomain.Intent) error {
	_, err := store.DB().ExecContext(ctx, `
		insert into execution_intents (
			id, kind, status, resource_type, resource_id, trace_id, message, last_error, claimed_by, claimed_at, lease_expires_at, attempt_count, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, intent.ID, intent.Kind, intent.Status, intent.ResourceType, intent.ResourceID, intent.TraceID, intent.Message, intent.LastError, intent.ClaimedBy, nullableTimePtr(intent.ClaimedAt), nullableTimePtr(intent.LeaseExpiresAt), intent.AttemptCount, intent.CreatedAt, intent.UpdatedAt, intent.DeletedAt)
	return err
}

func (s *intentService) UpdateStatus(ctx context.Context, id uuid.UUID, status model.IntentStatus, externalRef, message string) error {
	result, err := store.DB().ExecContext(ctx, `
		update execution_intents
		set status=$2, message=$3, last_error='', updated_at=$4
		where id = $1 and deleted_at is null
	`, id, status, message, time.Now())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *intentService) Get(ctx context.Context, id uuid.UUID) (*intentdomain.Intent, error) {
	return scanIntent(store.DB().QueryRowContext(ctx, `
		select id, kind, status, resource_type, resource_id, trace_id, message, last_error, claimed_by, claimed_at, lease_expires_at, attempt_count, created_at, updated_at, deleted_at
		from execution_intents
		where id = $1 and deleted_at is null
	`, id))
}

func (s *intentService) List(ctx context.Context, filter IntentListFilter) ([]*intentdomain.Intent, error) {
	query := `
		select id, kind, status, resource_type, resource_id, trace_id, message, last_error, claimed_by, claimed_at, lease_expires_at, attempt_count, created_at, updated_at, deleted_at
		from execution_intents
	`
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	if filter.Kind != "" {
		args = append(args, filter.Kind)
		clauses = append(clauses, placeholderClause("kind", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, placeholderClause("status", len(args)))
	}
	if filter.ResourceType != "" {
		args = append(args, filter.ResourceType)
		clauses = append(clauses, placeholderClause("resource_type", len(args)))
	}
	if filter.ClaimedBy != "" {
		args = append(args, filter.ClaimedBy)
		clauses = append(clauses, placeholderClause("claimed_by", len(args)))
	}
	if filter.ResourceID != nil {
		args = append(args, *filter.ResourceID)
		clauses = append(clauses, placeholderClause("resource_id", len(args)))
	}
	clauses = append(clauses, "deleted_at is null")
	query += " where " + strings.Join(clauses, " and ") + " order by created_at desc"

	rows, err := store.DB().QueryContext(ctx, query, args...)
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

func (s *intentService) ListPending(ctx context.Context, limit int) ([]intentdomain.Intent, error) {
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
	rows, err := store.DB().QueryContext(ctx, query, args...)
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

func (s *intentService) ClaimNextPending(ctx context.Context, workerID string, leaseDuration time.Duration) (*intentdomain.Intent, error) {
	now := time.Now()
	leaseExpiresAt := now.Add(leaseDuration)
	row := store.DB().QueryRowContext(ctx, `
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
	`, workerID, now, leaseExpiresAt, model.IntentPending)
	intent, err := scanIntent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrIntentNotFound
	}
	if err != nil {
		return nil, err
	}
	return intent, nil
}

func (s *intentService) MarkSubmitted(ctx context.Context, id uuid.UUID, externalRef, message string) error {
	result, err := store.DB().ExecContext(ctx, `
		update execution_intents
		set status=$2, message=$3, last_error='', updated_at=$4, claimed_by='', claimed_at=null, lease_expires_at=null
		where id = $1 and deleted_at is null
	`, id, model.IntentRunning, message, time.Now())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *intentService) MarkFailed(ctx context.Context, id uuid.UUID, message string) error {
	result, err := store.DB().ExecContext(ctx, `
		update execution_intents
		set status=$2, message=$3, last_error=$3, updated_at=$4, claimed_by='', claimed_at=null, lease_expires_at=null
		where id = $1 and deleted_at is null
	`, id, model.IntentFailed, message, time.Now())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *intentService) UpdateStatusByResource(ctx context.Context, kind model.IntentKind, resourceID uuid.UUID, status model.IntentStatus, externalRef, message string) error {
	result, err := store.DB().ExecContext(ctx, `
		update execution_intents
		set status=$3, message=$4, updated_at=$5
		where kind = $1 and resource_id = $2 and deleted_at is null
	`, kind, resourceID, status, message, time.Now())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *intentService) bindIntentToImage(ctx context.Context, imageID, intentID uuid.UUID) error {
	result, err := store.DB().ExecContext(ctx, `
		update images
		set execution_intent_id = $2, updated_at = $3
		where id = $1 and deleted_at is null
	`, imageID, intentID, time.Now())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *intentService) bindIntentToRelease(ctx context.Context, releaseID, intentID uuid.UUID) error {
	result, err := store.DB().ExecContext(ctx, `
		update releases
		set execution_intent_id = $2, updated_at = $3
		where id = $1 and deleted_at is null
	`, releaseID, intentID, time.Now())
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	v := id
	return &v
}
