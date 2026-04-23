package app

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bsonger/devflow-service/modules/meta-service/pkg/domain"
	"github.com/bsonger/devflow-service/shared/loggingx"
	"github.com/google/uuid"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const validClusterKubeConfig = `apiVersion: v1
kind: Config
clusters:
- name: prod
  cluster:
    server: https://kubernetes.example
users:
- name: prod-user
  user:
    token: devflow-token
contexts:
- name: prod
  context:
    cluster: prod
    user: prod-user
current-context: prod
`

type stubClusterOnboardingExecutor struct {
	errs  []error
	calls int
}

func (s *stubClusterOnboardingExecutor) Onboard(context.Context, *domain.Cluster) error {
	s.calls++
	idx := s.calls - 1
	if idx < len(s.errs) {
		return s.errs[idx]
	}
	return nil
}

func TestBuildArgoClusterSecretRejectsMalformedInputs(t *testing.T) {
	base := domain.Cluster{
		Name:              "prod",
		Server:            "https://kubernetes.example",
		KubeConfig:        validClusterKubeConfig,
		ArgoCDClusterName: "argocd-prod",
	}
	base.WithCreateDefault()

	tests := []struct {
		name   string
		mutate func(*domain.Cluster)
	}{
		{name: "missing id", mutate: func(c *domain.Cluster) { c.ID = uuid.Nil }},
		{name: "missing server", mutate: func(c *domain.Cluster) { c.Server = " " }},
		{name: "missing kubeconfig", mutate: func(c *domain.Cluster) { c.KubeConfig = " " }},
		{name: "invalid kubeconfig", mutate: func(c *domain.Cluster) { c.KubeConfig = "{" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := base
			tt.mutate(&cluster)

			_, err := buildArgoClusterSecret(&cluster)
			if !errors.Is(err, ErrClusterOnboardingMalformed) {
				t.Fatalf("buildArgoClusterSecret error = %v, want %v", err, ErrClusterOnboardingMalformed)
			}
		})
	}
}

func TestValidateArgoClusterSecretRejectsMalformedContract(t *testing.T) {
	tests := []struct {
		name   string
		secret *corev1.Secret
	}{
		{
			name: "missing argocd label",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster-a", Namespace: argoClusterSecretNamespace},
				StringData: map[string]string{"name": "cluster-a", "server": "https://kubernetes.example", "config": "{}"},
			},
		},
		{
			name: "missing server field",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-a",
					Namespace: argoClusterSecretNamespace,
					Labels:    map[string]string{argoClusterSecretTypeLabelKey: argoClusterSecretTypeLabelVal},
				},
				StringData: map[string]string{"name": "cluster-a", "config": "{}"},
			},
		},
		{
			name: "invalid config json",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-a",
					Namespace: argoClusterSecretNamespace,
					Labels:    map[string]string{argoClusterSecretTypeLabelKey: argoClusterSecretTypeLabelVal},
				},
				StringData: map[string]string{"name": "cluster-a", "server": "https://kubernetes.example", "config": "{"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArgoClusterSecret(tt.secret)
			if !errors.Is(err, ErrClusterOnboardingMalformed) {
				t.Fatalf("validateArgoClusterSecret error = %v, want %v", err, ErrClusterOnboardingMalformed)
			}
		})
	}
}

func TestClassifyClusterOnboardingUpsertError(t *testing.T) {
	forbidden := apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "cluster-a", errors.New("forbidden"))
	if err := classifyClusterOnboardingUpsertError(forbidden); !errors.Is(err, ErrClusterOnboardingFailed) || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("forbidden classification = %v", err)
	}

	conflict := apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, "cluster-a", errors.New("conflict"))
	if err := classifyClusterOnboardingUpsertError(conflict); !errors.Is(err, ErrClusterOnboardingFailed) || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("conflict classification = %v", err)
	}

	if err := classifyClusterOnboardingUpsertError(context.DeadlineExceeded); !errors.Is(err, ErrClusterOnboardingTimeout) {
		t.Fatalf("timeout classification = %v", err)
	}

	if err := classifyClusterOnboardingUpsertError(errors.New("dial tcp: i/o timeout")); !errors.Is(err, ErrClusterOnboardingFailed) || !strings.Contains(err.Error(), "transport") {
		t.Fatalf("transport classification = %v", err)
	}
}

func TestClusterCreatePersistsFailedOnboardingStatusAndRedactsError(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	stub := &queuedSQLDriverStub{execQueue: []queuedExecResponse{{result: driver.RowsAffected(1)}, {result: driver.RowsAffected(1)}}}
	openQueuedSQLStub(t, stub)

	onboarder := &stubClusterOnboardingExecutor{errs: []error{fmt.Errorf("%w: kubeconfig token leaked", ErrClusterOnboardingFailed)}}
	fixedNow := time.Date(2026, 4, 19, 3, 45, 0, 0, time.UTC)
	svc := newClusterService(onboarder, func() time.Time { return fixedNow })

	cluster := domain.Cluster{Name: "prod", Server: "https://kubernetes.example", KubeConfig: validClusterKubeConfig}
	cluster.WithCreateDefault()

	_, err := svc.Create(context.Background(), &cluster)
	if !errors.Is(err, ErrClusterOnboardingFailed) {
		t.Fatalf("Create error = %v, want %v", err, ErrClusterOnboardingFailed)
	}
	if cluster.OnboardingReady {
		t.Fatal("expected onboarding_ready=false after failed onboarding")
	}
	if cluster.OnboardingCheckedAt == nil || !cluster.OnboardingCheckedAt.Equal(fixedNow) {
		t.Fatalf("unexpected onboarding_checked_at: %#v", cluster.OnboardingCheckedAt)
	}
	if strings.Contains(strings.ToLower(cluster.OnboardingError), "token") || strings.Contains(strings.ToLower(err.Error()), "token") {
		t.Fatalf("onboarding error leaked sensitive material: %q / %q", cluster.OnboardingError, err.Error())
	}
	if len(stub.execs) != 2 {
		t.Fatalf("exec count = %d, want 2", len(stub.execs))
	}
	if !strings.Contains(stub.execs[1], "onboarding_ready") {
		t.Fatalf("expected onboarding status update query, got %q", stub.execs[1])
	}
}

func TestClusterCreateRetrySucceedsAfterPriorOnboardingFailure(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	stub := &queuedSQLDriverStub{}
	openQueuedSQLStub(t, stub)

	onboarder := &stubClusterOnboardingExecutor{errs: []error{errors.New("transient network failure"), nil}}
	svc := newClusterService(onboarder, time.Now)

	first := domain.Cluster{Name: "prod", Server: "https://kubernetes.example", KubeConfig: validClusterKubeConfig}
	first.WithCreateDefault()
	if _, err := svc.Create(context.Background(), &first); !errors.Is(err, ErrClusterOnboardingFailed) {
		t.Fatalf("first create error = %v, want %v", err, ErrClusterOnboardingFailed)
	}

	second := domain.Cluster{Name: "staging", Server: "https://staging.example", KubeConfig: validClusterKubeConfig}
	second.WithCreateDefault()
	id, err := svc.Create(context.Background(), &second)
	if err != nil {
		t.Fatalf("second create returned error: %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("expected second create to return cluster id")
	}
	if !second.OnboardingReady {
		t.Fatal("expected second create onboarding_ready=true")
	}
}

func TestClusterUpdateSkipsOnboardingWhenConnectionMaterialUnchanged(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	now := time.Now().UTC().Truncate(time.Second)
	clusterID := uuid.New()
	stub := &queuedSQLDriverStub{
		queryQueue: []queuedQueryResponse{{rows: queuedRows(
			[]string{"id", "name", "server", "kubeconfig", "argocd_cluster_name", "description", "labels", "onboarding_ready", "onboarding_error", "onboarding_checked_at", "created_at", "updated_at", "deleted_at"},
			[]driver.Value{clusterID.String(), "prod", "https://kubernetes.example", validClusterKubeConfig, "argocd-prod", "primary", []byte("[]"), true, "", now, now, now, nil},
		)}},
		execQueue: []queuedExecResponse{{result: driver.RowsAffected(1)}},
	}
	openQueuedSQLStub(t, stub)

	onboarder := &stubClusterOnboardingExecutor{}
	svc := newClusterService(onboarder, time.Now)

	cluster := domain.Cluster{BaseModel: domain.BaseModel{ID: clusterID}, Name: "prod", Server: "https://kubernetes.example", KubeConfig: validClusterKubeConfig, ArgoCDClusterName: "argocd-prod", Description: "updated"}
	if err := svc.Update(context.Background(), &cluster); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if onboarder.calls != 0 {
		t.Fatalf("onboard calls = %d, want 0", onboarder.calls)
	}
	if len(stub.execs) != 1 {
		t.Fatalf("exec count = %d, want 1", len(stub.execs))
	}
}

func TestClusterUpdatePersistsFailedOnboardingWhenConnectionMaterialChanges(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	now := time.Now().UTC().Truncate(time.Second)
	clusterID := uuid.New()
	stub := &queuedSQLDriverStub{
		queryQueue: []queuedQueryResponse{{rows: queuedRows(
			[]string{"id", "name", "server", "kubeconfig", "argocd_cluster_name", "description", "labels", "onboarding_ready", "onboarding_error", "onboarding_checked_at", "created_at", "updated_at", "deleted_at"},
			[]driver.Value{clusterID.String(), "prod", "https://kubernetes.example", validClusterKubeConfig, "argocd-prod", "primary", []byte("[]"), true, "", now, now, now, nil},
		)}},
		execQueue: []queuedExecResponse{{result: driver.RowsAffected(1)}, {result: driver.RowsAffected(1)}},
	}
	openQueuedSQLStub(t, stub)

	onboarder := &stubClusterOnboardingExecutor{errs: []error{apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, "cluster-a", errors.New("conflict"))}}
	svc := newClusterService(onboarder, time.Now)

	cluster := domain.Cluster{BaseModel: domain.BaseModel{ID: clusterID}, Name: "prod", Server: "https://new-kubernetes.example", KubeConfig: validClusterKubeConfig, ArgoCDClusterName: "argocd-prod", Description: "updated"}
	err := svc.Update(context.Background(), &cluster)
	if !errors.Is(err, ErrClusterOnboardingFailed) {
		t.Fatalf("Update error = %v, want %v", err, ErrClusterOnboardingFailed)
	}
	if cluster.OnboardingReady {
		t.Fatal("expected onboarding_ready=false after failed onboarding")
	}
	if cluster.OnboardingCheckedAt == nil {
		t.Fatal("expected onboarding_checked_at to be set")
	}
	if onboarder.calls != 1 {
		t.Fatalf("onboard calls = %d, want 1", onboarder.calls)
	}
	if len(stub.execs) != 2 {
		t.Fatalf("exec count = %d, want 2", len(stub.execs))
	}
	if !strings.Contains(stub.execs[1], "onboarding_ready") {
		t.Fatalf("expected onboarding status update query, got %q", stub.execs[1])
	}
}
