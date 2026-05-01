package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	"github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	"github.com/google/uuid"
)

type ListFilter struct {
	ApplicationID  *uuid.UUID
	IncludeDeleted bool
}

type Store interface {
	Create(context.Context, *domain.WorkloadConfig) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*domain.WorkloadConfig, error)
	Update(context.Context, *domain.WorkloadConfig) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, ListFilter) ([]domain.WorkloadConfig, error)
}

type PostgresStore struct{}

func NewPostgresStore() Store {
	return &PostgresStore{}
}

func (s *PostgresStore) Create(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error) {
	_, err := db.Postgres().ExecContext(ctx, `
		insert into workload_configs (
			id, application_id, replicas, service_account_name, resources, probes, env, labels, annotations, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, item.ID, item.ApplicationID, item.Replicas, dbsql.EmptyToNull(item.ServiceAccountName), dbsql.MustMarshalJSON(item.Resources, "{}"), dbsql.MustMarshalJSON(item.Probes, "{}"), dbsql.MustMarshalJSON(item.Env, "[]"), dbsql.MustMarshalJSON(item.Labels, "{}"), dbsql.MustMarshalJSON(item.Annotations, "{}"), item.CreatedAt, item.UpdatedAt, item.DeletedAt)
	if err != nil {
		return uuid.Nil, err
	}
	return item.ID, nil
}

func (s *PostgresStore) Get(ctx context.Context, id uuid.UUID) (*domain.WorkloadConfig, error) {
	item, normalized, err := scanWorkloadConfig(db.Postgres().QueryRowContext(ctx, `
		select id, application_id, replicas, service_account_name, resources, probes, env, labels, annotations, created_at, updated_at, deleted_at
		from workload_configs where id=$1 and deleted_at is null
	`, id))
	if err != nil {
		return nil, err
	}
	if !normalized.keep {
		if err := softDeleteWorkloadConfig(ctx, item.ID); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	if normalized.requiresRewrite {
		if err := persistNormalizedWorkloadConfig(ctx, item, normalized.updatedAt); err != nil {
			return nil, err
		}
	}
	return item, nil
}

func (s *PostgresStore) Update(ctx context.Context, item *domain.WorkloadConfig) error {
	result, err := db.Postgres().ExecContext(ctx, `
		update workload_configs
		set application_id=$2, replicas=$3, service_account_name=$4, resources=$5, probes=$6, env=$7, labels=$8, annotations=$9, updated_at=$10
		where id=$1 and deleted_at is null
	`, item.ID, item.ApplicationID, item.Replicas, dbsql.EmptyToNull(item.ServiceAccountName), dbsql.MustMarshalJSON(item.Resources, "{}"), dbsql.MustMarshalJSON(item.Probes, "{}"), dbsql.MustMarshalJSON(item.Env, "[]"), dbsql.MustMarshalJSON(item.Labels, "{}"), dbsql.MustMarshalJSON(item.Annotations, "{}"), item.UpdatedAt)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result, err := db.Postgres().ExecContext(ctx, `
		update workload_configs set deleted_at=$2, updated_at=$2
		where id=$1 and deleted_at is null
	`, id, now)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *PostgresStore) List(ctx context.Context, filter ListFilter) ([]domain.WorkloadConfig, error) {
	query := `
		select id, application_id, replicas, service_account_name, resources, probes, env, labels, annotations, created_at, updated_at, deleted_at
		from workload_configs
	`
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.ApplicationID != nil {
		args = append(args, *filter.ApplicationID)
		clauses = append(clauses, dbsql.PlaceholderClause("application_id", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"
	rows, err := db.Postgres().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var items []domain.WorkloadConfig
	var rewrites []*normalizedWorkloadConfig
	var deletions []uuid.UUID
	for rows.Next() {
		item, normalized, err := scanWorkloadConfig(rows)
		if err != nil {
			return nil, err
		}
		if !normalized.keep {
			deletions = append(deletions, item.ID)
			continue
		}
		if normalized.requiresRewrite {
			rewrites = append(rewrites, &normalizedWorkloadConfig{item: item, updatedAt: normalized.updatedAt})
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for _, pending := range rewrites {
		if err := persistNormalizedWorkloadConfig(ctx, pending.item, pending.updatedAt); err != nil {
			return nil, err
		}
	}
	for _, id := range deletions {
		if err := softDeleteWorkloadConfig(ctx, id); err != nil {
			return nil, err
		}
	}
	return items, nil
}

type workloadConfigRow struct {
	item               domain.WorkloadConfig
	serviceAccountName sql.NullString
	resourcesJSON      []byte
	probesJSON         []byte
	envJSON            []byte
	labelsJSON         []byte
	annotationsJSON    []byte
	deletedAt          sql.NullTime
}

type normalizationDecision struct {
	keep            bool
	requiresRewrite bool
	updatedAt       time.Time
}

type normalizedWorkloadConfig struct {
	item      *domain.WorkloadConfig
	updatedAt time.Time
}

func scanWorkloadConfig(scanner interface{ Scan(dest ...any) error }) (*domain.WorkloadConfig, normalizationDecision, error) {
	row, err := scanWorkloadConfigRow(scanner)
	if err != nil {
		return nil, normalizationDecision{}, err
	}
	item, normalized, err := normalizeWorkloadConfigRow(row)
	if err != nil {
		return nil, normalizationDecision{}, err
	}
	return item, normalized, nil
}

func scanWorkloadConfigRow(scanner interface{ Scan(dest ...any) error }) (*workloadConfigRow, error) {
	var row workloadConfigRow
	if err := scanner.Scan(&row.item.ID, &row.item.ApplicationID, &row.item.Replicas, &row.serviceAccountName, &row.resourcesJSON, &row.probesJSON, &row.envJSON, &row.labelsJSON, &row.annotationsJSON, &row.item.CreatedAt, &row.item.UpdatedAt, &row.deletedAt); err != nil {
		return nil, err
	}
	if row.serviceAccountName.Valid {
		row.item.ServiceAccountName = row.serviceAccountName.String
	}
	row.item.DeletedAt = dbsql.TimePtrFromNull(row.deletedAt)
	return &row, nil
}

func normalizeWorkloadConfigRow(row *workloadConfigRow) (*domain.WorkloadConfig, normalizationDecision, error) {
	item := row.item
	normalized := normalizationDecision{keep: true}

	resources, changed, keep, err := decodeLegacyResources(row.resourcesJSON)
	if err != nil {
		return nil, normalizationDecision{}, scanWorkloadConfigLegacyError(item.ID, "resources")
	}
	if !keep {
		return &item, normalizationDecision{keep: false}, nil
	}
	item.Resources = resources
	if changed {
		normalized.requiresRewrite = true
	}

	probes, changed, keep, err := decodeLegacyProbes(row.probesJSON)
	if err != nil {
		return nil, normalizationDecision{}, scanWorkloadConfigLegacyError(item.ID, "probes")
	}
	if !keep {
		return &item, normalizationDecision{keep: false}, nil
	}
	item.Probes = probes
	if changed {
		normalized.requiresRewrite = true
	}

	env, changed, keep, err := decodeLegacyEnv(row.envJSON)
	if err != nil {
		return nil, normalizationDecision{}, scanWorkloadConfigLegacyError(item.ID, "env")
	}
	if !keep {
		return &item, normalizationDecision{keep: false}, nil
	}
	item.Env = env
	if changed {
		normalized.requiresRewrite = true
	}

	labels, changed, err := decodeStringMapJSON(row.labelsJSON)
	if err != nil {
		return nil, normalizationDecision{}, scanWorkloadConfigLegacyError(item.ID, "labels")
	}
	item.Labels = labels
	if changed {
		normalized.requiresRewrite = true
	}

	annotations, changed, err := decodeStringMapJSON(row.annotationsJSON)
	if err != nil {
		return nil, normalizationDecision{}, scanWorkloadConfigLegacyError(item.ID, "annotations")
	}
	item.Annotations = annotations
	if changed {
		normalized.requiresRewrite = true
	}

	if normalized.requiresRewrite {
		normalized.updatedAt = time.Now()
		item.UpdatedAt = normalized.updatedAt
	}
	return &item, normalized, nil
}

func decodeLegacyResources(payload []byte) (domain.WorkloadResourceRequirements, bool, bool, error) {
	if isEmptyJSONObject(payload) {
		return domain.WorkloadResourceRequirements{}, true, false, nil
	}

	var typed domain.WorkloadResourceRequirements
	if err := json.Unmarshal(payload, &typed); err == nil {
		if typed.SizeClass != "" {
			mapped, ok := domain.WorkloadSizeClassResources[typed.SizeClass]
			if !ok {
				return domain.WorkloadResourceRequirements{}, false, false, nil
			}
			if hasResourceListValues(typed.Requests) || hasResourceListValues(typed.Limits) {
				if !resourcesEqualLists(typed.Requests, mapped.Requests) || !resourcesEqualLists(typed.Limits, mapped.Limits) {
					return domain.WorkloadResourceRequirements{}, false, false, nil
				}
			}
			return mapped, !resourcesExactlyCanonical(typed, mapped), true, nil
		}
	}

	var legacy struct {
		Requests map[string]string `json:"requests"`
		Limits   map[string]string `json:"limits"`
	}
	if err := json.Unmarshal(payload, &legacy); err != nil {
		return domain.WorkloadResourceRequirements{}, false, false, nil
	}
	if len(legacy.Requests) == 0 && len(legacy.Limits) == 0 {
		return domain.WorkloadResourceRequirements{}, true, false, nil
	}
	for sizeClass, mapped := range domain.WorkloadSizeClassResources {
		if resourceMapMatches(legacy.Requests, mapped.Requests) && resourceMapMatches(legacy.Limits, mapped.Limits) {
			mapped.SizeClass = sizeClass
			return mapped, true, true, nil
		}
	}
	return domain.WorkloadResourceRequirements{}, false, false, nil
}

func decodeLegacyProbes(payload []byte) (domain.WorkloadProbes, bool, bool, error) {
	if isEmptyJSONObject(payload) {
		return domain.WorkloadProbes{}, false, true, nil
	}
	var probes domain.WorkloadProbes
	if err := json.Unmarshal(payload, &probes); err == nil {
		return probes, false, true, nil
	}
	return domain.WorkloadProbes{}, false, false, nil
}

func decodeLegacyEnv(payload []byte) ([]domain.EnvVar, bool, bool, error) {
	if isEmptyJSONArray(payload) {
		return nil, false, true, nil
	}
	var env []domain.EnvVar
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, false, false, nil
	}
	if len(env) == 0 {
		return nil, false, true, nil
	}
	seen := make(map[string]struct{}, len(env))
	for _, entry := range env {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			return nil, false, false, nil
		}
		if _, ok := seen[name]; ok {
			return nil, false, false, nil
		}
		seen[name] = struct{}{}
	}
	return env, false, true, nil
}

func decodeStringMapJSON(payload []byte) (map[string]string, bool, error) {
	if isEmptyJSONObject(payload) {
		return nil, false, nil
	}
	var values map[string]string
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil, false, err
	}
	if len(values) == 0 {
		return nil, true, nil
	}
	return values, false, nil
}

func persistNormalizedWorkloadConfig(ctx context.Context, item *domain.WorkloadConfig, updatedAt time.Time) error {
	// Legacy row cleanup belongs at the repository boundary so manifest/release consumers only ever
	// observe the constrained model or a missing row, never ad-hoc compatibility branches.
	result, err := db.Postgres().ExecContext(ctx, `
		update workload_configs
		set resources=$2, probes=$3, env=$4, labels=$5, annotations=$6, updated_at=$7
		where id=$1 and deleted_at is null
	`, item.ID, dbsql.MustMarshalJSON(item.Resources, "{}"), dbsql.MustMarshalJSON(item.Probes, "{}"), dbsql.MustMarshalJSON(item.Env, "[]"), dbsql.MustMarshalJSON(item.Labels, "{}"), dbsql.MustMarshalJSON(item.Annotations, "{}"), updatedAt)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func softDeleteWorkloadConfig(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result, err := db.Postgres().ExecContext(ctx, `
		update workload_configs set deleted_at=$2, updated_at=$2
		where id=$1 and deleted_at is null
	`, id, now)
	if err != nil {
		return err
	}
	if err := dbsql.EnsureRowsAffected(result); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	return nil
}

func isEmptyJSONObject(payload []byte) bool {
	trimmed := strings.TrimSpace(string(payload))
	return trimmed == "" || trimmed == "null" || trimmed == "{}"
}

func isEmptyJSONArray(payload []byte) bool {
	trimmed := strings.TrimSpace(string(payload))
	return trimmed == "" || trimmed == "null" || trimmed == "[]"
}

func resourceMapMatches(values map[string]string, want domain.WorkloadResourceList) bool {
	if len(values) != 2 {
		return false
	}
	return strings.TrimSpace(values["cpu"]) == want.CPU && strings.TrimSpace(values["memory"]) == want.Memory
}

func hasResourceListValues(list domain.WorkloadResourceList) bool {
	return strings.TrimSpace(list.CPU) != "" || strings.TrimSpace(list.Memory) != ""
}

func resourcesEqualLists(got, want domain.WorkloadResourceList) bool {
	return strings.TrimSpace(got.CPU) == want.CPU && strings.TrimSpace(got.Memory) == want.Memory
}

func resourcesExactlyCanonical(got, want domain.WorkloadResourceRequirements) bool {
	return got.SizeClass == want.SizeClass && resourcesEqualLists(got.Requests, want.Requests) && resourcesEqualLists(got.Limits, want.Limits)
}

func scanWorkloadConfigLegacyError(id uuid.UUID, field string) error {
	return fmt.Errorf("workload-config %s legacy %s decode failed", id.String(), field)
}

var _ Store = (*PostgresStore)(nil)
