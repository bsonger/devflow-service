package app

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	loggingx "github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/domain"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/infra/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var ClusterService = NewClusterService()

const (
	argoClusterSecretNamespace    = "argocd"
	argoClusterSecretTypeLabelKey = "argocd.argoproj.io/secret-type"
	argoClusterSecretTypeLabelVal = "cluster"
	clusterSecretIDLabelKey       = "devflow.io/cluster-id"
	clusterOnboardingFieldManager = "devflow-meta-service"
)

var (
	ErrClusterNameRequired         = errors.New("cluster name is required")
	ErrClusterServerRequired       = errors.New("cluster server is required")
	ErrClusterKubeConfigRequired   = errors.New("cluster kubeconfig is required")
	ErrClusterConflict             = errors.New("cluster already exists")
	ErrClusterOnboardingFailed     = errors.New("cluster onboarding failed")
	ErrClusterOnboardingTimeout    = errors.New("cluster onboarding timed out")
	ErrClusterOnboardingMalformed  = errors.New("cluster onboarding payload malformed")
	ErrClusterOnboardingStatusSync = errors.New("cluster onboarding status persistence failed")
)

type ClusterListFilter struct {
	IncludeDeleted bool
	Name           string
}

type clusterOnboardingExecutor interface {
	Onboard(ctx context.Context, cluster *domain.Cluster) error
}

type clusterService struct {
	onboarding clusterOnboardingExecutor
	now        func() time.Time
}

func NewClusterService() *clusterService {
	return newClusterService(newKubernetesClusterOnboarder(), time.Now)
}

func newClusterService(onboarding clusterOnboardingExecutor, now func() time.Time) *clusterService {
	if onboarding == nil {
		onboarding = noopClusterOnboardingExecutor{}
	}
	if now == nil {
		now = time.Now
	}
	return &clusterService{onboarding: onboarding, now: now}
}

type noopClusterOnboardingExecutor struct{}

func (noopClusterOnboardingExecutor) Onboard(context.Context, *domain.Cluster) error { return nil }

type kubernetesClusterOnboarder struct {
	secretClientFactory func() (corev1typed.SecretInterface, error)
}

func newKubernetesClusterOnboarder() clusterOnboardingExecutor {
	return &kubernetesClusterOnboarder{
		secretClientFactory: func() (corev1typed.SecretInterface, error) {
			cfg, err := rest.InClusterConfig()
			if err != nil {
				return nil, classifyClusterOnboardingClientError(err)
			}
			clientset, err := kubernetes.NewForConfig(cfg)
			if err != nil {
				return nil, classifyClusterOnboardingClientError(err)
			}
			return clientset.CoreV1().Secrets(argoClusterSecretNamespace), nil
		},
	}
}

func (o *kubernetesClusterOnboarder) Onboard(ctx context.Context, cluster *domain.Cluster) error {
	secret, err := buildArgoClusterSecret(cluster)
	if err != nil {
		return err
	}
	if err := validateArgoClusterSecret(secret); err != nil {
		return err
	}

	secretClient, err := o.secretClientFactory()
	if err != nil {
		return err
	}

	payload, err := json.Marshal(secret)
	if err != nil {
		return fmt.Errorf("%w: unable to encode secret payload", ErrClusterOnboardingMalformed)
	}

	force := true
	_, err = secretClient.Patch(
		ctx,
		secret.Name,
		types.ApplyPatchType,
		payload,
		metav1.PatchOptions{FieldManager: clusterOnboardingFieldManager, Force: &force},
	)
	if err != nil {
		return classifyClusterOnboardingUpsertError(err)
	}

	return nil
}

func (s *clusterService) Create(ctx context.Context, cluster *domain.Cluster) (uuid.UUID, error) {
	log := clusterLogger(ctx, "create_cluster", cluster.GetID())

	if err := validateCluster(cluster); err != nil {
		logClusterFailure(log, "create cluster failed", err)
		return uuid.Nil, err
	}

	labels, err := marshalLabels(cluster.Labels)
	if err != nil {
		logClusterFailure(log, "marshal cluster labels failed", err)
		return uuid.Nil, err
	}

	_, err = store.DB().ExecContext(ctx, `
		insert into clusters (
			id, name, server, kubeconfig, argocd_cluster_name, description, labels,
			onboarding_ready, onboarding_error, onboarding_checked_at,
			created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, cluster.ID, cluster.Name, cluster.Server, cluster.KubeConfig, cluster.ArgoCDClusterName, cluster.Description, labels, false, "", nil, cluster.CreatedAt, cluster.UpdatedAt, cluster.DeletedAt)
	if err != nil {
		if isUniqueViolation(err) {
			logClusterFailure(log, "create cluster conflict", ErrClusterConflict)
			return uuid.Nil, ErrClusterConflict
		}
		logClusterFailure(log, "create cluster failed", err)
		return uuid.Nil, err
	}

	onboardingErr := s.runClusterOnboarding(ctx, cluster)
	if onboardingErr != nil {
		logClusterFailure(log, "cluster onboarding failed", onboardingErr)
		return uuid.Nil, onboardingErr
	}

	log.Info("cluster created",
		zap.String("result", "success"),
		zap.String("cluster_name", cluster.Name),
		zap.String("server", cluster.Server),
		zap.Bool("onboarding_ready", cluster.OnboardingReady),
	)
	return cluster.GetID(), nil
}

func (s *clusterService) Get(ctx context.Context, id uuid.UUID) (*domain.Cluster, error) {
	log := clusterLogger(ctx, "get_cluster", id)

	cluster, err := scanCluster(store.DB().QueryRowContext(ctx, `
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

func (s *clusterService) Update(ctx context.Context, cluster *domain.Cluster) error {
	log := clusterLogger(ctx, "update_cluster", cluster.GetID())

	current, err := s.Get(ctx, cluster.GetID())
	if err != nil {
		logClusterFailure(log, "load cluster failed", err)
		return err
	}

	cluster.CreatedAt = current.CreatedAt
	cluster.DeletedAt = current.DeletedAt
	cluster.WithUpdateDefault()

	if err := validateCluster(cluster); err != nil {
		logClusterFailure(log, "update cluster failed", err)
		return err
	}

	labels, err := marshalLabels(cluster.Labels)
	if err != nil {
		logClusterFailure(log, "marshal cluster labels failed", err)
		return err
	}

	result, err := store.DB().ExecContext(ctx, `
		update clusters
		set name=$2, server=$3, kubeconfig=$4, argocd_cluster_name=$5, description=$6, labels=$7, updated_at=$8, deleted_at=$9
		where id = $1 and deleted_at is null
	`, cluster.ID, cluster.Name, cluster.Server, cluster.KubeConfig, cluster.ArgoCDClusterName, cluster.Description, labels, cluster.UpdatedAt, cluster.DeletedAt)
	if err != nil {
		if isUniqueViolation(err) {
			logClusterFailure(log, "update cluster conflict", ErrClusterConflict)
			return ErrClusterConflict
		}
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

	cluster.OnboardingReady = current.OnboardingReady
	cluster.OnboardingError = current.OnboardingError
	cluster.OnboardingCheckedAt = current.OnboardingCheckedAt
	if requiresClusterOnboarding(current, cluster) {
		onboardingErr := s.runClusterOnboarding(ctx, cluster)
		if onboardingErr != nil {
			logClusterFailure(log, "cluster onboarding failed", onboardingErr)
			return onboardingErr
		}
	}

	log.Info("cluster updated",
		zap.String("result", "success"),
		zap.String("cluster_name", cluster.Name),
		zap.String("server", cluster.Server),
		zap.Bool("onboarding_ready", cluster.OnboardingReady),
	)
	return nil
}

func (s *clusterService) Delete(ctx context.Context, id uuid.UUID) error {
	log := clusterLogger(ctx, "delete_cluster", id)

	now := time.Now()
	result, err := store.DB().ExecContext(ctx, `
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

	log.Info("cluster deleted", zap.String("result", "success"))
	return nil
}

func (s *clusterService) List(ctx context.Context, filter ClusterListFilter) ([]domain.Cluster, error) {
	log := loggingx.LoggerWithContext(ctx).With(
		zap.String("operation", "list_clusters"),
		zap.String("resource", "cluster"),
		zap.String("result", "started"),
		zap.Any("filter", filter),
	)

	query := `
		select id, name, server, kubeconfig, argocd_cluster_name, description, labels,
			onboarding_ready, onboarding_error, onboarding_checked_at,
			created_at, updated_at, deleted_at
		from clusters
	`
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)

	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at is null")
	}
	if filter.Name != "" {
		args = append(args, strings.TrimSpace(filter.Name))
		clauses = append(clauses, placeholderClause("name", len(args)))
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc"

	rows, err := store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		logClusterFailure(log, "list clusters failed", err)
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	clusters := make([]domain.Cluster, 0)
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

func (s *clusterService) runClusterOnboarding(ctx context.Context, cluster *domain.Cluster) error {
	checkedAt := s.now().UTC()
	onboardingErr := normalizeOnboardingError(s.onboarding.Onboard(ctx, cluster))

	status := clusterOnboardingStatusFromResult(onboardingErr, checkedAt)
	if persistErr := s.persistOnboardingStatus(ctx, cluster.ID, status); persistErr != nil {
		return persistErr
	}

	cluster.OnboardingReady = status.Ready
	cluster.OnboardingError = status.Error
	cluster.OnboardingCheckedAt = &status.CheckedAt
	return onboardingErr
}

func (s *clusterService) persistOnboardingStatus(ctx context.Context, clusterID uuid.UUID, status clusterOnboardingStatus) error {
	if clusterID == uuid.Nil {
		return fmt.Errorf("%w: cluster id is required", ErrClusterOnboardingStatusSync)
	}

	result, err := store.DB().ExecContext(ctx, `
		update clusters
		set onboarding_ready=$2, onboarding_error=$3, onboarding_checked_at=$4, updated_at=$5
		where id = $1 and deleted_at is null
	`, clusterID, status.Ready, status.Error, status.CheckedAt, s.now())
	if err != nil {
		return fmt.Errorf("%w: %s", ErrClusterOnboardingStatusSync, err.Error())
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: read result failed", ErrClusterOnboardingStatusSync)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

type clusterOnboardingStatus struct {
	Ready     bool
	Error     string
	CheckedAt time.Time
}

func clusterOnboardingStatusFromResult(onboardingErr error, checkedAt time.Time) clusterOnboardingStatus {
	status := clusterOnboardingStatus{Ready: onboardingErr == nil, CheckedAt: checkedAt}
	if onboardingErr != nil {
		status.Error = sanitizeOnboardingError(onboardingErr)
	}
	return status
}

func sanitizeOnboardingError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrClusterOnboardingTimeout) {
		return "cluster onboarding timed out while upserting argo cluster secret"
	}

	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "cluster onboarding failed"
	}
	if containsSensitiveMaterial(msg) {
		return "cluster onboarding failed: sensitive detail redacted"
	}
	if len(msg) > 512 {
		msg = msg[:512]
	}
	return msg
}

func containsSensitiveMaterial(msg string) bool {
	lower := strings.ToLower(msg)
	sensitiveMarkers := []string{
		"apiVersion:",
		"client-certificate-data",
		"client-key-data",
		"authorization:",
		"bearer",
		"token",
		"password",
		"-----begin",
	}
	for _, marker := range sensitiveMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func normalizeOnboardingError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, ErrClusterOnboardingMalformed) {
		if containsSensitiveMaterial(err.Error()) {
			return fmt.Errorf("%w: cluster onboarding payload malformed", ErrClusterOnboardingMalformed)
		}
		return err
	}
	if errors.Is(err, ErrClusterOnboardingTimeout) {
		if containsSensitiveMaterial(err.Error()) {
			return fmt.Errorf("%w: argo cluster secret upsert timed out", ErrClusterOnboardingTimeout)
		}
		return err
	}
	if errors.Is(err, ErrClusterOnboardingFailed) {
		if containsSensitiveMaterial(err.Error()) {
			return fmt.Errorf("%w: cluster onboarding execution failed", ErrClusterOnboardingFailed)
		}
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: argo cluster secret upsert timed out", ErrClusterOnboardingTimeout)
	}
	return fmt.Errorf("%w: cluster onboarding execution failed", ErrClusterOnboardingFailed)
}

func requiresClusterOnboarding(current, next *domain.Cluster) bool {
	if current == nil || next == nil {
		return true
	}
	return strings.TrimSpace(current.Name) != strings.TrimSpace(next.Name) ||
		strings.TrimSpace(current.Server) != strings.TrimSpace(next.Server) ||
		strings.TrimSpace(current.KubeConfig) != strings.TrimSpace(next.KubeConfig) ||
		strings.TrimSpace(current.ArgoCDClusterName) != strings.TrimSpace(next.ArgoCDClusterName)
}

func buildArgoClusterSecret(cluster *domain.Cluster) (*corev1.Secret, error) {
	if cluster == nil {
		return nil, fmt.Errorf("%w: cluster payload is required", ErrClusterOnboardingMalformed)
	}
	if cluster.ID == uuid.Nil {
		return nil, fmt.Errorf("%w: cluster id is required", ErrClusterOnboardingMalformed)
	}

	displayName := strings.TrimSpace(cluster.ArgoCDClusterName)
	if displayName == "" {
		displayName = strings.TrimSpace(cluster.Name)
	}
	if displayName == "" {
		return nil, fmt.Errorf("%w: cluster name is required", ErrClusterOnboardingMalformed)
	}

	server := strings.TrimSpace(cluster.Server)
	if server == "" {
		return nil, fmt.Errorf("%w: cluster server is required", ErrClusterOnboardingMalformed)
	}

	kubeconfig := strings.TrimSpace(cluster.KubeConfig)
	if kubeconfig == "" {
		return nil, fmt.Errorf("%w: cluster kubeconfig is required", ErrClusterOnboardingMalformed)
	}

	configJSON, err := buildArgoClusterConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      argoClusterSecretName(cluster.ID),
			Namespace: argoClusterSecretNamespace,
			Labels: map[string]string{
				argoClusterSecretTypeLabelKey: argoClusterSecretTypeLabelVal,
				clusterSecretIDLabelKey:       cluster.ID.String(),
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"name":   displayName,
			"server": server,
			"config": configJSON,
		},
	}

	if err := validateArgoClusterSecret(secret); err != nil {
		return nil, err
	}

	return secret, nil
}

func validateArgoClusterSecret(secret *corev1.Secret) error {
	if secret == nil {
		return fmt.Errorf("%w: secret payload is required", ErrClusterOnboardingMalformed)
	}
	if strings.TrimSpace(secret.Namespace) == "" {
		return fmt.Errorf("%w: secret namespace is required", ErrClusterOnboardingMalformed)
	}
	if strings.TrimSpace(secret.Name) == "" {
		return fmt.Errorf("%w: secret name is required", ErrClusterOnboardingMalformed)
	}
	if strings.TrimSpace(secret.Labels[argoClusterSecretTypeLabelKey]) != argoClusterSecretTypeLabelVal {
		return fmt.Errorf("%w: argocd cluster secret label is required", ErrClusterOnboardingMalformed)
	}

	requiredFields := []string{"name", "server", "config"}
	for _, field := range requiredFields {
		if strings.TrimSpace(secret.StringData[field]) == "" {
			return fmt.Errorf("%w: secret field %s is required", ErrClusterOnboardingMalformed, field)
		}
	}
	if !json.Valid([]byte(secret.StringData["config"])) {
		return fmt.Errorf("%w: secret field config must be valid JSON", ErrClusterOnboardingMalformed)
	}

	return nil
}

func buildArgoClusterConfigFromKubeConfig(kubeconfig string) (string, error) {
	restCfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return "", fmt.Errorf("%w: invalid kubeconfig payload", ErrClusterOnboardingMalformed)
	}

	config := map[string]any{}
	if restCfg.BearerToken != "" {
		config["bearerToken"] = restCfg.BearerToken
	}
	if restCfg.Username != "" {
		config["username"] = restCfg.Username
	}
	if restCfg.Password != "" {
		config["password"] = restCfg.Password
	}

	tlsConfig := map[string]any{"insecure": restCfg.Insecure}
	if len(restCfg.CAData) > 0 {
		tlsConfig["caData"] = base64.StdEncoding.EncodeToString(restCfg.CAData)
	}
	if len(restCfg.CertData) > 0 {
		tlsConfig["certData"] = base64.StdEncoding.EncodeToString(restCfg.CertData)
	}
	if len(restCfg.KeyData) > 0 {
		tlsConfig["keyData"] = base64.StdEncoding.EncodeToString(restCfg.KeyData)
	}
	if restCfg.ServerName != "" {
		tlsConfig["serverName"] = restCfg.ServerName
	}
	if len(tlsConfig) > 0 {
		config["tlsClientConfig"] = tlsConfig
	}

	payload, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("%w: unable to encode cluster config", ErrClusterOnboardingMalformed)
	}
	return string(payload), nil
}

func argoClusterSecretName(clusterID uuid.UUID) string {
	return "devflow-cluster-" + strings.ToLower(clusterID.String())
}

func classifyClusterOnboardingClientError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%w: kubernetes client initialization timed out", ErrClusterOnboardingTimeout)
	default:
		return fmt.Errorf("%w: kubernetes client initialization failed", ErrClusterOnboardingFailed)
	}
}

func classifyClusterOnboardingUpsertError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.DeadlineExceeded), apierrors.IsTimeout(err), apierrors.IsServerTimeout(err):
		return fmt.Errorf("%w: argo cluster secret upsert timed out", ErrClusterOnboardingTimeout)
	case apierrors.IsForbidden(err):
		return fmt.Errorf("%w: argo cluster secret upsert forbidden", ErrClusterOnboardingFailed)
	case apierrors.IsConflict(err):
		return fmt.Errorf("%w: argo cluster secret upsert conflict", ErrClusterOnboardingFailed)
	default:
		return fmt.Errorf("%w: argo cluster secret upsert transport failure", ErrClusterOnboardingFailed)
	}
}

func validateCluster(cluster *domain.Cluster) error {
	cluster.Name = strings.TrimSpace(cluster.Name)
	cluster.Server = strings.TrimSpace(cluster.Server)
	cluster.ArgoCDClusterName = strings.TrimSpace(cluster.ArgoCDClusterName)

	if cluster.Name == "" {
		return ErrClusterNameRequired
	}
	if cluster.Server == "" {
		return ErrClusterServerRequired
	}
	if strings.TrimSpace(cluster.KubeConfig) == "" {
		return ErrClusterKubeConfigRequired
	}
	return nil
}

func scanCluster(scanner interface {
	Scan(dest ...any) error
}) (*domain.Cluster, error) {
	var (
		cluster             domain.Cluster
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

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func appErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, sql.ErrNoRows):
		return "not_found"
	case errors.Is(err, ErrClusterConflict), errors.Is(err, ErrEnvironmentConflict):
		return "conflict"
	case errors.Is(err, ErrClusterNameRequired),
		errors.Is(err, ErrClusterServerRequired),
		errors.Is(err, ErrClusterKubeConfigRequired),
		errors.Is(err, ErrEnvironmentNameRequired),
		errors.Is(err, ErrEnvironmentClusterRequired),
		errors.Is(err, ErrClusterReferenceNotFound),
		errors.Is(err, ErrClusterOnboardingMalformed):
		return "invalid_argument"
	case errors.Is(err, ErrClusterOnboardingTimeout):
		return "deadline_exceeded"
	case errors.Is(err, ErrClusterOnboardingFailed):
		return "failed_precondition"
	default:
		return "internal"
	}
}
