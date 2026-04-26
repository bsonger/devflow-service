package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	platformdb "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/google/uuid"
)

type Store interface {
	CreateRuntimeSpec(context.Context, *runtimedomain.RuntimeSpec) error
	GetRuntimeSpec(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error)
	GetRuntimeSpecByApplicationEnv(context.Context, uuid.UUID, string) (*runtimedomain.RuntimeSpec, error)
	DeleteRuntimeSpecByApplicationEnv(context.Context, uuid.UUID, string) error
	ListRuntimeSpecs(context.Context) ([]*runtimedomain.RuntimeSpec, error)
	NextRevisionNumber(context.Context, uuid.UUID) (int, error)
	CreateRuntimeSpecRevision(context.Context, *runtimedomain.RuntimeSpecRevision) error
	UpdateCurrentRevision(context.Context, uuid.UUID, uuid.UUID) error
	ListRuntimeSpecRevisions(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeSpecRevision, error)
	GetRuntimeSpecRevision(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpecRevision, error)
	UpsertObservedPod(context.Context, *runtimedomain.RuntimeObservedPod) error
	DeleteObservedPod(context.Context, uuid.UUID, string, string, time.Time) error
	ListObservedPods(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error)
	CreateRuntimeOperation(context.Context, *runtimedomain.RuntimeOperation) error
	ListRuntimeOperations(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeOperation, error)
}

var RuntimeStore Store = NewPostgresStore()

type postgresStore struct{}

func NewPostgresStore() Store {
	return &postgresStore{}
}

func (s *postgresStore) CreateRuntimeSpec(ctx context.Context, spec *runtimedomain.RuntimeSpec) error {
	_, err := platformdb.Postgres().ExecContext(ctx, `
		insert into application_runtime_specs (
			id, application_id, environment, current_revision_id, created_at, updated_at
		) values ($1, $2, $3, $4, $5, $6)
	`, spec.ID, spec.ApplicationID, spec.Environment, dbsql.NullableUUIDPtr(spec.CurrentRevisionID), spec.CreatedAt, spec.UpdatedAt)
	return err
}

func (s *postgresStore) GetRuntimeSpec(ctx context.Context, id uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
	return scanRuntimeSpec(platformdb.Postgres().QueryRowContext(ctx, `
		select id, application_id, environment, current_revision_id, created_at, updated_at
		from application_runtime_specs
		where id = $1
	`, id))
}

func (s *postgresStore) GetRuntimeSpecByApplicationEnv(ctx context.Context, applicationId uuid.UUID, environment string) (*runtimedomain.RuntimeSpec, error) {
	spec, err := scanRuntimeSpec(platformdb.Postgres().QueryRowContext(ctx, `
		select id, application_id, environment, current_revision_id, created_at, updated_at
		from application_runtime_specs
		where application_id = $1 and environment = $2
	`, applicationId, environment))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return spec, err
}

func (s *postgresStore) DeleteRuntimeSpecByApplicationEnv(ctx context.Context, applicationId uuid.UUID, environment string) error {
	result, err := platformdb.Postgres().ExecContext(ctx, `
		delete from application_runtime_specs
		where application_id = $1 and environment = $2
	`, applicationId, environment)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *postgresStore) ListRuntimeSpecs(ctx context.Context) ([]*runtimedomain.RuntimeSpec, error) {
	rows, err := platformdb.Postgres().QueryContext(ctx, `
		select id, application_id, environment, current_revision_id, created_at, updated_at
		from application_runtime_specs
		order by created_at desc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*runtimedomain.RuntimeSpec, 0)
	for rows.Next() {
		item, scanErr := scanRuntimeSpec(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *postgresStore) NextRevisionNumber(ctx context.Context, runtimeSpecID uuid.UUID) (int, error) {
	var next int
	err := platformdb.Postgres().QueryRowContext(ctx, `
		select coalesce(max(revision), 0) + 1
		from application_runtime_spec_revisions
		where runtime_spec_id = $1
	`, runtimeSpecID).Scan(&next)
	return next, err
}

func (s *postgresStore) CreateRuntimeSpecRevision(ctx context.Context, revision *runtimedomain.RuntimeSpecRevision) error {
	healthThresholds, err := marshalJSONText(revision.HealthThresholds, "{}")
	if err != nil {
		return err
	}
	resources, err := marshalJSONText(revision.Resources, "{}")
	if err != nil {
		return err
	}
	autoscaling, err := marshalJSONText(revision.Autoscaling, "{}")
	if err != nil {
		return err
	}
	scheduling, err := marshalJSONText(revision.Scheduling, "{}")
	if err != nil {
		return err
	}
	podEnvs, err := marshalJSONText(revision.PodEnvs, "[]")
	if err != nil {
		return err
	}

	_, err = platformdb.Postgres().ExecContext(ctx, `
		insert into application_runtime_spec_revisions (
			id, runtime_spec_id, revision, replicas,
			health_thresholds_jsonb, resources_jsonb, autoscaling_jsonb, scheduling_jsonb,
			pod_envs_jsonb, created_by, created_at
		) values (
			$1, $2, $3, $4,
			$5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb,
			$9::jsonb, $10, $11
		)
	`, revision.ID, revision.RuntimeSpecID, revision.Revision, revision.Replicas,
		healthThresholds, resources, autoscaling, scheduling,
		podEnvs, revision.CreatedBy, revision.CreatedAt)
	return err
}

func (s *postgresStore) UpdateCurrentRevision(ctx context.Context, runtimeSpecID uuid.UUID, revisionID uuid.UUID) error {
	result, err := platformdb.Postgres().ExecContext(ctx, `
		update application_runtime_specs
		set current_revision_id = $2, updated_at = $3
		where id = $1
	`, runtimeSpecID, revisionID, time.Now().UTC())
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *postgresStore) ListRuntimeSpecRevisions(ctx context.Context, runtimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeSpecRevision, error) {
	rows, err := platformdb.Postgres().QueryContext(ctx, `
		select id, runtime_spec_id, revision, replicas,
			health_thresholds_jsonb, resources_jsonb, autoscaling_jsonb, scheduling_jsonb,
			pod_envs_jsonb, created_by, created_at
		from application_runtime_spec_revisions
		where runtime_spec_id = $1
		order by revision desc
	`, runtimeSpecID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*runtimedomain.RuntimeSpecRevision, 0)
	for rows.Next() {
		item, scanErr := scanRuntimeSpecRevision(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *postgresStore) GetRuntimeSpecRevision(ctx context.Context, id uuid.UUID) (*runtimedomain.RuntimeSpecRevision, error) {
	return scanRuntimeSpecRevision(platformdb.Postgres().QueryRowContext(ctx, `
		select id, runtime_spec_id, revision, replicas,
			health_thresholds_jsonb, resources_jsonb, autoscaling_jsonb, scheduling_jsonb,
			pod_envs_jsonb, created_by, created_at
		from application_runtime_spec_revisions
		where id = $1
	`, id))
}

func (s *postgresStore) UpsertObservedPod(ctx context.Context, pod *runtimedomain.RuntimeObservedPod) error {
	labels, err := marshalJSONText(pod.Labels, "{}")
	if err != nil {
		return err
	}
	containers, err := marshalJSONText(pod.Containers, "[]")
	if err != nil {
		return err
	}

	_, err = platformdb.Postgres().ExecContext(ctx, `
		insert into runtime_observed_pods (
			id, runtime_spec_id, application_id, environment, namespace, pod_name,
			phase, ready, restarts, node_name, pod_ip, host_ip, owner_kind, owner_name,
			labels_jsonb, containers_jsonb, observed_at, deleted_at
		) values (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13, $14,
			$15::jsonb, $16::jsonb, $17, $18
		)
		on conflict (runtime_spec_id, namespace, pod_name) do update set
			id = excluded.id,
			application_id = excluded.application_id,
			environment = excluded.environment,
			phase = excluded.phase,
			ready = excluded.ready,
			restarts = excluded.restarts,
			node_name = excluded.node_name,
			pod_ip = excluded.pod_ip,
			host_ip = excluded.host_ip,
			owner_kind = excluded.owner_kind,
			owner_name = excluded.owner_name,
			labels_jsonb = excluded.labels_jsonb,
			containers_jsonb = excluded.containers_jsonb,
			observed_at = excluded.observed_at,
			deleted_at = excluded.deleted_at
	`, pod.ID, pod.RuntimeSpecID, pod.ApplicationID, pod.Environment, pod.Namespace, pod.PodName,
		pod.Phase, pod.Ready, pod.Restarts, pod.NodeName, pod.PodIP, pod.HostIP, pod.OwnerKind, pod.OwnerName,
		labels, containers, pod.ObservedAt, dbsql.NullableTimePtr(pod.DeletedAt))
	return err
}

func (s *postgresStore) DeleteObservedPod(ctx context.Context, runtimeSpecID uuid.UUID, namespace, podName string, observedAt time.Time) error {
	result, err := platformdb.Postgres().ExecContext(ctx, `
		update runtime_observed_pods
		set deleted_at = $4, observed_at = $4
		where runtime_spec_id = $1 and namespace = $2 and pod_name = $3 and deleted_at is null
	`, runtimeSpecID, namespace, podName, observedAt)
	if err != nil {
		return err
	}
	return dbsql.EnsureRowsAffected(result)
}

func (s *postgresStore) ListObservedPods(ctx context.Context, runtimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error) {
	rows, err := platformdb.Postgres().QueryContext(ctx, `
		select id, runtime_spec_id, application_id, environment, namespace, pod_name,
			phase, ready, restarts, node_name, pod_ip, host_ip, owner_kind, owner_name,
			labels_jsonb, containers_jsonb, observed_at, deleted_at
		from runtime_observed_pods
		where runtime_spec_id = $1 and deleted_at is null
		order by observed_at desc, pod_name asc
	`, runtimeSpecID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*runtimedomain.RuntimeObservedPod, 0)
	for rows.Next() {
		item, scanErr := scanObservedPod(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *postgresStore) CreateRuntimeOperation(ctx context.Context, op *runtimedomain.RuntimeOperation) error {
	_, err := platformdb.Postgres().ExecContext(ctx, `
		insert into runtime_operations (
			id, runtime_spec_id, operation_type, target_name, operator, created_at
		) values ($1, $2, $3, $4, $5, $6)
	`, op.ID, op.RuntimeSpecID, op.OperationType, op.TargetName, op.Operator, op.CreatedAt)
	return err
}

func (s *postgresStore) ListRuntimeOperations(ctx context.Context, runtimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeOperation, error) {
	rows, err := platformdb.Postgres().QueryContext(ctx, `
		select id, runtime_spec_id, operation_type, target_name, operator, created_at
		from runtime_operations
		where runtime_spec_id = $1
		order by created_at desc
	`, runtimeSpecID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*runtimedomain.RuntimeOperation, 0)
	for rows.Next() {
		item, scanErr := scanRuntimeOperation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanRuntimeSpec(scanner interface{ Scan(dest ...any) error }) (*runtimedomain.RuntimeSpec, error) {
	var (
		item              runtimedomain.RuntimeSpec
		currentRevisionID sql.NullString
	)
	if err := scanner.Scan(
		&item.ID,
		&item.ApplicationID,
		&item.Environment,
		&currentRevisionID,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	parsedRevisionID, err := dbsql.ParseNullUUID(currentRevisionID)
	if err != nil {
		return nil, err
	}
	item.CurrentRevisionID = parsedRevisionID
	return &item, nil
}

func scanRuntimeSpecRevision(scanner interface{ Scan(dest ...any) error }) (*runtimedomain.RuntimeSpecRevision, error) {
	var (
		item             runtimedomain.RuntimeSpecRevision
		healthThresholds []byte
		resources        []byte
		autoscaling      []byte
		scheduling       []byte
		podEnvs          []byte
	)
	if err := scanner.Scan(
		&item.ID,
		&item.RuntimeSpecID,
		&item.Revision,
		&item.Replicas,
		&healthThresholds,
		&resources,
		&autoscaling,
		&scheduling,
		&podEnvs,
		&item.CreatedBy,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	item.HealthThresholds = normalizeJSONText(healthThresholds, "{}")
	item.Resources = normalizeJSONText(resources, "{}")
	item.Autoscaling = normalizeJSONText(autoscaling, "{}")
	item.Scheduling = normalizeJSONText(scheduling, "{}")
	item.PodEnvs = normalizeJSONText(podEnvs, "[]")
	return &item, nil
}

func scanRuntimeOperation(scanner interface{ Scan(dest ...any) error }) (*runtimedomain.RuntimeOperation, error) {
	var item runtimedomain.RuntimeOperation
	if err := scanner.Scan(
		&item.ID,
		&item.RuntimeSpecID,
		&item.OperationType,
		&item.TargetName,
		&item.Operator,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanObservedPod(scanner interface{ Scan(dest ...any) error }) (*runtimedomain.RuntimeObservedPod, error) {
	var (
		item       runtimedomain.RuntimeObservedPod
		labels     []byte
		containers []byte
		deletedAt  sql.NullTime
	)
	if err := scanner.Scan(
		&item.ID,
		&item.RuntimeSpecID,
		&item.ApplicationID,
		&item.Environment,
		&item.Namespace,
		&item.PodName,
		&item.Phase,
		&item.Ready,
		&item.Restarts,
		&item.NodeName,
		&item.PodIP,
		&item.HostIP,
		&item.OwnerKind,
		&item.OwnerName,
		&labels,
		&containers,
		&item.ObservedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(defaultJSONBytes(labels, `{}`), &item.Labels); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(defaultJSONBytes(containers, `[]`), &item.Containers); err != nil {
		return nil, err
	}
	item.DeletedAt = dbsql.TimePtrFromNull(deletedAt)
	return &item, nil
}

func marshalJSONText(value any, empty string) (string, error) {
	payload, err := dbsql.MarshalJSON(value, empty)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func normalizeJSONText(raw []byte, empty string) string {
	if len(raw) == 0 {
		return empty
	}
	return string(raw)
}

func defaultJSONBytes(raw []byte, empty string) []byte {
	if len(raw) == 0 {
		return []byte(empty)
	}
	return raw
}
