package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bsonger/devflow-service/modules/meta-service/pkg/app"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubClusterService struct {
	createFn func(context.Context, *domain.Cluster) (uuid.UUID, error)
	getFn    func(context.Context, uuid.UUID) (*domain.Cluster, error)
	updateFn func(context.Context, *domain.Cluster) error
	deleteFn func(context.Context, uuid.UUID) error
	listFn   func(context.Context, app.ClusterListFilter) ([]domain.Cluster, error)
}

func (s stubClusterService) Create(ctx context.Context, cluster *domain.Cluster) (uuid.UUID, error) {
	return s.createFn(ctx, cluster)
}
func (s stubClusterService) Get(ctx context.Context, id uuid.UUID) (*domain.Cluster, error) {
	return s.getFn(ctx, id)
}
func (s stubClusterService) Update(ctx context.Context, cluster *domain.Cluster) error {
	return s.updateFn(ctx, cluster)
}
func (s stubClusterService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.deleteFn(ctx, id)
}
func (s stubClusterService) List(ctx context.Context, filter app.ClusterListFilter) ([]domain.Cluster, error) {
	return s.listFn(ctx, filter)
}

func TestCreateClusterReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	checkedAt := time.Now().UTC().Truncate(time.Second)
	handler := &ClusterHandler{svc: stubClusterService{createFn: func(_ context.Context, cluster *domain.Cluster) (uuid.UUID, error) {
		cluster.OnboardingReady = true
		cluster.OnboardingError = ""
		cluster.OnboardingCheckedAt = &checkedAt
		return cluster.GetID(), nil
	}}}

	r := gin.New()
	r.POST("/api/v1/clusters", handler.Create)

	body := bytes.NewBufferString(`{"name":"prod","server":"https://kubernetes.example","kubeconfig":"apiVersion: v1","argocd_cluster_name":"argocd-prod","description":"primary cluster","labels":[{"key":"team","value":"platform"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d want %d", rec.Code, http.StatusCreated)
	}

	var payload struct {
		Data domain.Cluster `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Data.Name != "prod" || payload.Data.Server != "https://kubernetes.example" || payload.Data.ArgoCDClusterName != "argocd-prod" {
		t.Fatalf("unexpected payload: %#v", payload.Data)
	}
	if !payload.Data.OnboardingReady || payload.Data.OnboardingCheckedAt == nil {
		t.Fatalf("missing onboarding readiness fields in payload: %#v", payload.Data)
	}
}

func TestCreateClusterConflictReturnsErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ClusterHandler{svc: stubClusterService{createFn: func(_ context.Context, _ *domain.Cluster) (uuid.UUID, error) {
		return uuid.Nil, app.ErrClusterConflict
	}}}

	r := gin.New()
	r.POST("/api/v1/clusters", handler.Create)

	body := bytes.NewBufferString(`{"name":"prod","server":"https://kubernetes.example","kubeconfig":"apiVersion: v1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Error.Code != "conflict" {
		t.Fatalf("error code = %q, want conflict", payload.Error.Code)
	}
}

func TestCreateClusterOnboardingFailureReturnsFailedPrecondition(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ClusterHandler{svc: stubClusterService{createFn: func(_ context.Context, _ *domain.Cluster) (uuid.UUID, error) {
		return uuid.Nil, app.ErrClusterOnboardingFailed
	}}}

	r := gin.New()
	r.POST("/api/v1/clusters", handler.Create)

	body := bytes.NewBufferString(`{"name":"prod","server":"https://kubernetes.example","kubeconfig":"apiVersion: v1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "failed_precondition") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestGetClusterNotFoundReturnsErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ClusterHandler{svc: stubClusterService{getFn: func(_ context.Context, _ uuid.UUID) (*domain.Cluster, error) {
		return nil, sql.ErrNoRows
	}}}

	r := gin.New()
	r.GET("/api/v1/clusters/:id", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNotFound)
	}
}

func TestListClustersReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ClusterHandler{svc: stubClusterService{listFn: func(_ context.Context, filter app.ClusterListFilter) ([]domain.Cluster, error) {
		if filter.Name != "prod" {
			t.Fatalf("unexpected filter: %#v", filter)
		}
		return []domain.Cluster{{Name: "prod", Server: "https://kubernetes.example", OnboardingReady: true}}, nil
	}}}

	r := gin.New()
	r.GET("/api/v1/clusters", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters?name=prod&page=1&page_size=20", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Data []domain.Cluster `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].Name != "prod" || !payload.Data[0].OnboardingReady {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestCreateClusterValidationErrorReturnsInvalidArgument(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ClusterHandler{svc: stubClusterService{createFn: func(_ context.Context, _ *domain.Cluster) (uuid.UUID, error) {
		return uuid.Nil, app.ErrClusterNameRequired
	}}}

	r := gin.New()
	r.POST("/api/v1/clusters", handler.Create)

	body := bytes.NewBufferString(`{"name":"","server":"https://kubernetes.example","kubeconfig":"apiVersion: v1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invalid_argument") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestWriteClusterErrorMapsOnboardingTimeout(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/timeout", func(c *gin.Context) { writeClusterError(c, app.ErrClusterOnboardingTimeout) })

	req := httptest.NewRequest(http.MethodGet, "/timeout", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusGatewayTimeout, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "deadline_exceeded") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestWriteClusterErrorFallsBackToInternal(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/boom", func(c *gin.Context) { writeClusterError(c, errors.New("boom")) })

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d want %d", rec.Code, http.StatusInternalServerError)
	}
}
