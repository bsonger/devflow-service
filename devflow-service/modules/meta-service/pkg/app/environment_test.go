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

func TestEnvironmentCreateRejectsBlankNameAndMissingClusterID(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	openQueuedSQLStub(t, &queuedSQLDriverStub{})

	tests := []struct {
		name        string
		environment domain.Environment
		wantErr     error
	}{
		{
			name:        "missing name",
			environment: domain.Environment{ClusterID: uuid.New()},
			wantErr:     ErrEnvironmentNameRequired,
		},
		{
			name:        "missing cluster id",
			environment: domain.Environment{Name: "staging"},
			wantErr:     ErrEnvironmentClusterRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			environment := tt.environment
			environment.WithCreateDefault()
			_, err := NewEnvironmentService().Create(context.Background(), &environment)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Create error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnvironmentCreateRejectsMissingClusterReference(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	stub := &queuedSQLDriverStub{
		queryQueue: []queuedQueryResponse{{terr: sql.ErrNoRows}},
	}
	openQueuedSQLStub(t, stub)

	environment := domain.Environment{Name: "staging", ClusterID: uuid.New()}
	environment.WithCreateDefault()

	_, err := NewEnvironmentService().Create(context.Background(), &environment)
	if !errors.Is(err, ErrClusterReferenceNotFound) {
		t.Fatalf("Create error = %v, want %v", err, ErrClusterReferenceNotFound)
	}
	if len(stub.queries) != 1 || !strings.Contains(stub.queries[0], "from clusters") {
		t.Fatalf("unexpected queries: %#v", stub.queries)
	}
}

func TestEnvironmentCreateReturnsConflictOnDuplicateName(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	now := time.Now()
	clusterID := uuid.New()
	stub := &queuedSQLDriverStub{
		queryQueue: []queuedQueryResponse{{rows: queuedRows(
			[]string{"id", "name", "server", "kubeconfig", "argocd_cluster_name", "description", "labels", "onboarding_ready", "onboarding_error", "onboarding_checked_at", "created_at", "updated_at", "deleted_at"},
			[]driver.Value{clusterID.String(), "prod", "https://kubernetes.example", "apiVersion: v1", "", "primary", []byte("[]"), true, "", now, now, now, nil},
		)}},
		execQueue: []queuedExecResponse{{err: &pgconn.PgError{Code: "23505"}}},
	}
	openQueuedSQLStub(t, stub)

	environment := domain.Environment{Name: "staging", ClusterID: clusterID}
	environment.WithCreateDefault()

	_, err := NewEnvironmentService().Create(context.Background(), &environment)
	if !errors.Is(err, ErrEnvironmentConflict) {
		t.Fatalf("Create error = %v, want %v", err, ErrEnvironmentConflict)
	}
}

func TestEnvironmentGetRejectsMalformedClusterIDFromStorage(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	now := time.Now()
	stub := &queuedSQLDriverStub{
		queryQueue: []queuedQueryResponse{{rows: queuedRows(
			[]string{"id", "name", "cluster_id", "description", "labels", "created_at", "updated_at", "deleted_at"},
			[]driver.Value{uuid.New().String(), "staging", "not-a-uuid", "target", []byte("[]"), now, now, nil},
		)}},
	}
	openQueuedSQLStub(t, stub)

	_, err := NewEnvironmentService().Get(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("Get should fail when cluster_id cannot be parsed")
	}
}

func TestEnvironmentListFiltersByClusterID(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	now := time.Now()
	clusterID := uuid.New()
	stub := &queuedSQLDriverStub{
		queryQueue: []queuedQueryResponse{{rows: queuedRows(
			[]string{"id", "name", "cluster_id", "description", "labels", "created_at", "updated_at", "deleted_at"},
			[]driver.Value{uuid.New().String(), "production", clusterID.String(), "primary", []byte(`[{"key":"tier","value":"prod"}]`), now, now, nil},
		)}},
	}
	openQueuedSQLStub(t, stub)

	environments, err := NewEnvironmentService().List(context.Background(), EnvironmentListFilter{ClusterID: &clusterID})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(environments) != 1 || environments[0].ClusterID != clusterID {
		t.Fatalf("unexpected environments: %#v", environments)
	}
	if len(stub.queries) != 1 || !strings.Contains(stub.queries[0], "cluster_id") {
		t.Fatalf("unexpected queries: %#v", stub.queries)
	}
}

func TestEnvironmentDeleteReturnsNotFoundWhenRowMissing(t *testing.T) {
	loggingx.Logger = zap.NewNop()
	stub := &queuedSQLDriverStub{
		execQueue: []queuedExecResponse{{result: driver.RowsAffected(0)}},
	}
	openQueuedSQLStub(t, stub)

	err := NewEnvironmentService().Delete(context.Background(), uuid.New())
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Delete error = %v, want %v", err, sql.ErrNoRows)
	}
}
