package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	clusterdomain "github.com/bsonger/devflow-service/internal/cluster/domain"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	argoClusterSecretNamespace    = "argocd"
	argoClusterSecretTypeLabelKey = "argocd.argoproj.io/secret-type"
	argoClusterSecretTypeLabelVal = "cluster"
	clusterSecretIDLabelKey       = "devflow.io/cluster-id"
	clusterOnboardingFieldManager = "devflow-meta-service"
)

type clusterOnboardingExecutor interface {
	Onboard(context.Context, *clusterdomain.Cluster) error
}

type noopClusterOnboardingExecutor struct{}

func (noopClusterOnboardingExecutor) Onboard(context.Context, *clusterdomain.Cluster) error {
	return nil
}

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

func (o *kubernetesClusterOnboarder) Onboard(ctx context.Context, cluster *clusterdomain.Cluster) error {
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
	_, err = secretClient.Patch(ctx, secret.Name, types.ApplyPatchType, payload, metav1.PatchOptions{
		FieldManager: clusterOnboardingFieldManager,
		Force:        &force,
	})
	if err != nil {
		return classifyClusterOnboardingUpsertError(err)
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
	for _, marker := range []string{
		"apiversion:",
		"client-certificate-data",
		"client-key-data",
		"authorization:",
		"bearer",
		"token",
		"password",
		"-----begin",
	} {
		if strings.Contains(lower, marker) {
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

func requiresClusterOnboarding(current, next *clusterdomain.Cluster) bool {
	if current == nil || next == nil {
		return true
	}
	return strings.TrimSpace(current.Name) != strings.TrimSpace(next.Name) ||
		strings.TrimSpace(current.Server) != strings.TrimSpace(next.Server) ||
		strings.TrimSpace(current.KubeConfig) != strings.TrimSpace(next.KubeConfig) ||
		strings.TrimSpace(current.ArgoCDClusterName) != strings.TrimSpace(next.ArgoCDClusterName)
}

func buildArgoClusterSecret(cluster *clusterdomain.Cluster) (*corev1.Secret, error) {
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
	for _, field := range []string{"name", "server", "config"} {
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
