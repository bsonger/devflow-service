package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bsonger/devflow-service/modules/meta-service/pkg/domain"
	"github.com/bsonger/devflow-service/shared/loggingx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

func TestClusterCreateRejectsBlankRequiredFields(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	openQueuedSQLStub(t, &queuedSQLDriverStub{})

	tests := []struct {
		name    string
		cluster domain.Cluster
		wantErr error
	}{
		{
			name:    "missing name",
			cluster: domain.Cluster{Server: "https://kubernetes.example", KubeConfig: "apiVersion: v1"},
			wantErr: ErrClusterNameRequired,
		},
		{
			name:    "missing server",
			cluster: domain.Cluster{Name: "prod", KubeConfig: "apiVersion: v1"},
			wantErr: ErrClusterServerRequired,
		},
		{
			name:    "missing kubeconfig",
			cluster: domain.Cluster{Name: "prod", Server: "https://kubernetes.example"},
			wantErr: ErrClusterKubeConfigRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := tt.cluster
			cluster.WithCreateDefault()
			_, err := NewClusterService().Create(context.Background(), &cluster)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Create error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestClusterCreateReturnsConflictOnDuplicateName(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	stub := &queuedSQLDriverStub{
		execQueue: []queuedExecResponse{{err: &pgconn.PgError{Code: "23505"}}},
	}
	openQueuedSQLStub(t, stub)

	cluster := domain.Cluster{Name: "prod", Server: "https://kubernetes.example", KubeConfig: "apiVersion: v1"}
	cluster.WithCreateDefault()

	_, err := NewClusterService().Create(context.Background(), &cluster)
	if !errors.Is(err, ErrClusterConflict) {
		t.Fatalf("Create error = %v, want %v", err, ErrClusterConflict)
	}
	if len(stub.execs) != 1 {
		t.Fatalf("exec count = %d, want 1", len(stub.execs))
	}
}

func TestClusterGetRejectsMalformedLabelsFromStorage(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	now := time.Now()
	stub := &queuedSQLDriverStub{
		queryQueue: []queuedQueryResponse{{rows: queuedRows(
			[]string{"id", "name", "server", "kubeconfig", "argocd_cluster_name", "description", "labels", "onboarding_ready", "onboarding_error", "onboarding_checked_at", "created_at", "updated_at", "deleted_at"},
			[]driver.Value{uuid.New().String(), "prod", "https://kubernetes.example", "apiVersion: v1", "argocd-prod", "primary", []byte("{"), false, "", nil, now, now, nil},
		)}},
	}
	openQueuedSQLStub(t, stub)

	_, err := NewClusterService().Get(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("Get should fail when labels JSON is malformed")
	}
}

func TestClusterListReturnsRowsAndLegacyLabels(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	now := time.Now()
	stub := &queuedSQLDriverStub{
		queryQueue: []queuedQueryResponse{{rows: queuedRows(
			[]string{"id", "name", "server", "kubeconfig", "argocd_cluster_name", "description", "labels", "onboarding_ready", "onboarding_error", "onboarding_checked_at", "created_at", "updated_at", "deleted_at"},
			[]driver.Value{uuid.New().String(), "prod", "https://kubernetes.example", "apiVersion: v1", "argocd-prod", "primary", []byte(`[{"key":"team","value":"platform"}]`), true, "", now, now, now, nil},
			[]driver.Value{uuid.New().String(), "staging", "https://staging.example", "apiVersion: v1", "", "staging", []byte(`{"team":"delivery"}`), false, "sync failed", now, now, now, nil},
		)}},
	}
	openQueuedSQLStub(t, stub)

	clusters, err := NewClusterService().List(context.Background(), ClusterListFilter{Name: "prod"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("cluster count = %d, want 2", len(clusters))
	}
	if clusters[0].Labels[0].Key != "team" || clusters[1].Labels[0].Value != "delivery" {
		t.Fatalf("unexpected labels: %#v", clusters)
	}
	if len(stub.queries) != 1 || !strings.Contains(stub.queries[0], "from clusters") {
		t.Fatalf("unexpected queries: %#v", stub.queries)
	}
}

func TestClusterDeleteReturnsNotFoundWhenRowMissing(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	stub := &queuedSQLDriverStub{
		execQueue: []queuedExecResponse{{result: driver.RowsAffected(0)}},
	}
	openQueuedSQLStub(t, stub)

	err := NewClusterService().Delete(context.Background(), uuid.New())
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Delete error = %v, want %v", err, sql.ErrNoRows)
	}
}
