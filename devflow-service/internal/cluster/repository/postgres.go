package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	clusterdomain "github.com/bsonger/devflow-service/internal/cluster/domain"
	platformdb "github.com/bsonger/devflow-service/internal/platform/db"
	loggingx "github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

type Store interface {
	Create(context.Context, *clusterdomain.Cluster) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*clusterdomain.Cluster, error)
	Update(context.Context, *clusterdomain.Cluster) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, bool, string) ([]clusterdomain.Cluster, error)
	PersistOnboardingStatus(context.Context, uuid.UUID, bool, string, time.Time, time.Time) error
}

var ClusterStore Store = NewPostgresStore()

type postgresStore struct{}

func NewPostgresStore() Store {
	return &postgresStore{}
}

func (s *postgresStore) Create(ctx context.Context, cluster *clusterdomain.Cluster) (uuid.UUID, error) {
	log := clusterLogger(ctx, "create_cluster", cluster.GetID())

	labels, err := marshalLabels(cluster.Labels)
	if err != nil {
		logClusterFailure(log, "marshal cluster labels failed", err)
		return uuid.Nil, err
	}

	_, err = platformdb.Postgres().ExecContext(ctx, `
		insert into clusters (
			id, name, server, kubeconfig, argocd_cluster_name, description, labels,
			onboarding_ready, onboarding_error, onboarding_checked_at,
			created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, cluster.ID, cluster.Name, cluster.Server, cluster.KubeConfig, cluster.ArgoCDClusterName, cluster.Description, labels, false, "", nil, cluster.CreatedAt, cluster.UpdatedAt, cluster.DeletedAt)
	if err != nil {
		logClusterFailure(log, "create cluster failed", err)
		return uuid.Nil, err
	}

	return cluster.GetID(), nil
}

func (s *postgresStore) Get(ctx context.Context, id uuid.UUID) (*clusterdomain.Cluster, error) {
	log := clusterLogger(ctx, "get_cluster", id)

	cluster, err := scanCluster(platformdb.Postgres().QueryRowContext(ctx, `
		select id, name, server, kubeconfig, argocd_cluster_name, description, labels,
			onboarding_ready, onboarding_error, onboarding_checked_at,
			created_at, updated_at, deleted_at
		from clusters
		where id = $1 and deleted_at is null
	`, id))
	if err != nil {
		logClusterFailure(log, "get cluster failed", err)
		return nil, err
	}

	log.Debug("cluster fetched",
		zap.String("result", "success"),
		zap.String("cluster_name", cluster.Name),
		zap.String("server", cluster.Server),
		zap.Bool("onboarding_ready", cluster.OnboardingReady),
	)
	return cluster, nil
}

func (s *postgresStore) Update(ctx context.Context, cluster *clusterdomain.Cluster) error {
	log := clusterLogger(ctx, "update_cluster", cluster.GetID())

	current, err := s.Get(ctx, cluster.GetID())
	if err != nil {
		logClusterFailure(log, "load cluster failed", err)
		return err
	}

	cluster.CreatedAt = current.CreatedAt
	cluster.DeletedAt = current.DeletedAt
	cluster.WithUpdateDefault()

	labels, err := marshalLabels(cluster.Labels)
	if err != nil {
		logClusterFailure(log, "marshal cluster labels failed", err)
		return err
	}

	result, err := platformdb.Postgres().ExecContext(ctx, `
		update clusters
		set name=$2, server=$3, kubeconfig=$4, argocd_cluster_name=$5, description=$6, labels=$7, updated_at=$8, deleted_at=$9
		where id = $1 and deleted_at is null
	`, cluster.ID, cluster.Name, cluster.Server, cluster.KubeConfig, cluster.ArgoCDClusterName, cluster.Description, labels, cluster.UpdatedAt, cluster.DeletedAt)
	if err != nil {
		logClusterFailure(log, "update cluster failed", err)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		logClusterFailure(log, "read cluster update result failed", err)
		return err
	}
	if rows == 0 {
		logClusterFailure(log, "update cluster missed row", sql.ErrNoRows)
		return sql.ErrNoRows
	}

	return nil
}

func (s *postgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	log := clusterLogger(ctx, "delete_cluster", id)

	now := time.Now()
	result, err := platformdb.Postgres().ExecContext(ctx, `
		update clusters
		set deleted_at=$2, updated_at=$2
		where id = $1 and deleted_at is null
	`, id, now)
	if err != nil {
		logClusterFailure(log, "delete cluster failed", err)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		logClusterFailure(log, "read cluster delete result failed", err)
		return err
	}
	if rows == 0 {
		logClusterFailure(log, "delete cluster missed row", sql.ErrNoRows)
		return sql.ErrNoRows
	}

	return nil
}

func (s *postgresStore) List(ctx context.Context, includeDeleted bool, name string) ([]clusterdomain.Cluster, error) {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "list_clusters"),
		zap.String("resource", "cluster"),
		zap.String("result", "started"),
		zap.Bool("include_deleted", includeDeleted),
		zap.String("name", name),
	)

	query := `
		select id, name, server, kubeconfig, argocd_cluster_name, description, labels,
			onboarding_ready, onboarding_error, onboarding_checked_at,
			created_at, updated_at, deleted_at
		from clusters
	`
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)

	if !includeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if name != "" {
		args = append(args, strings.TrimSpace(name))
		clauses = append(clauses, placeholderClause("name", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"

	rows, err := platformdb.Postgres().QueryContext(ctx, query, args...)
	if err != nil {
		logClusterFailure(log, "list clusters failed", err)
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	clusters := make([]clusterdomain.Cluster, 0)
	for rows.Next() {
		cluster, err := scanCluster(rows)
		if err != nil {
			logClusterFailure(log, "scan cluster failed", err)
			return nil, err
		}
		clusters = append(clusters, *cluster)
	}
	if err := rows.Err(); err != nil {
		logClusterFailure(log, "iterate clusters failed", err)
		return nil, err
	}

	log.Debug("clusters listed", zap.String("result", "success"), zap.Int("count", len(clusters)))
	return clusters, nil
}

func (s *postgresStore) PersistOnboardingStatus(ctx context.Context, clusterID uuid.UUID, ready bool, errText string, checkedAt, updatedAt time.Time) error {
	result, err := platformdb.Postgres().ExecContext(ctx, `
		update clusters
		set onboarding_ready=$2, onboarding_error=$3, onboarding_checked_at=$4, updated_at=$5
		where id = $1 and deleted_at is null
	`, clusterID, ready, errText, checkedAt, updatedAt)
	if err != nil {
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

func scanCluster(scanner interface {
	Scan(dest ...any) error
}) (*clusterdomain.Cluster, error) {
	var (
		cluster             clusterdomain.Cluster
		labels              []byte
		onboardingError     sql.NullString
		onboardingCheckedAt sql.NullTime
		deletedAt           sql.NullTime
	)

	if err := scanner.Scan(
		&cluster.ID,
		&cluster.Name,
		&cluster.Server,
		&cluster.KubeConfig,
		&cluster.ArgoCDClusterName,
		&cluster.Description,
		&labels,
		&cluster.OnboardingReady,
		&onboardingError,
		&onboardingCheckedAt,
		&cluster.CreatedAt,
		&cluster.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}

	if onboardingError.Valid {
		cluster.OnboardingError = onboardingError.String
	}
	if onboardingCheckedAt.Valid {
		cluster.OnboardingCheckedAt = &onboardingCheckedAt.Time
	}
	if deletedAt.Valid {
		cluster.DeletedAt = &deletedAt.Time
	}
	if len(labels) > 0 {
		parsed, err := unmarshalLabels(labels)
		if err != nil {
			return nil, err
		}
		cluster.Labels = parsed
	}

	return &cluster, nil
}

func marshalLabels(labels []clusterdomain.LabelItem) ([]byte, error) {
	if labels == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(labels)
}

func unmarshalLabels(raw []byte) ([]clusterdomain.LabelItem, error) {
	var labels []clusterdomain.LabelItem
	if err := json.Unmarshal(raw, &labels); err == nil {
		return labels, nil
	}
	var legacy map[string]string
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, err
	}
	labels = make([]clusterdomain.LabelItem, 0, len(legacy))
	for key, value := range legacy {
		labels = append(labels, clusterdomain.LabelItem{Key: key, Value: value})
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i].Key < labels[j].Key })
	return labels, nil
}

func placeholderClause(column string, position int) string {
	return column + " = $" + strconv.Itoa(position)
}

func AsPgConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func clusterLogger(ctx context.Context, operation string, resourceID uuid.UUID) *zap.Logger {
	resourceIDValue := ""
	if resourceID != uuid.Nil {
		resourceIDValue = resourceID.String()
	}
	return loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", operation),
		zap.String("resource", "cluster"),
		zap.String("resource_id", resourceIDValue),
	)
}

func logClusterFailure(log *zap.Logger, msg string, err error) {
	log.Error(msg,
		zap.String("result", "error"),
		zap.String("error_code", appErrorCode(err)),
		zap.Error(err),
	)
}

func appErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, sql.ErrNoRows):
		return "not_found"
	default:
		return "internal"
	}
}
