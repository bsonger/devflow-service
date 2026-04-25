package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	workloadconfig "github.com/bsonger/devflow-service/internal/workloadconfig/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type mockWorkloadConfigService struct {
	createFunc func(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error)
	getFunc    func(ctx context.Context, id uuid.UUID) (*domain.WorkloadConfig, error)
	updateFunc func(ctx context.Context, item *domain.WorkloadConfig) error
	deleteFunc func(ctx context.Context, id uuid.UUID) error
	listFunc   func(ctx context.Context, filter workloadconfig.WorkloadConfigListFilter) ([]domain.WorkloadConfig, error)
}

func (m *mockWorkloadConfigService) Create(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, item)
	}
	return uuid.New(), nil
}

func (m *mockWorkloadConfigService) Get(ctx context.Context, id uuid.UUID) (*domain.WorkloadConfig, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockWorkloadConfigService) Update(ctx context.Context, item *domain.WorkloadConfig) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, item)
	}
	return nil
}

func (m *mockWorkloadConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *mockWorkloadConfigService) List(ctx context.Context, filter workloadconfig.WorkloadConfigListFilter) ([]domain.WorkloadConfig, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, filter)
	}
	return nil, nil
}

func setupTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)
	return r
}

func TestCreateWorkloadConfig(t *testing.T) {
	wlSvc := &mockWorkloadConfigService{}
	h := NewHandler(wlSvc)
	r := setupTestRouter(h)

	reqBody, _ := json.Marshal(domain.WorkloadConfigInput{ApplicationID: uuid.New(), Name: "test-wl", Replicas: 1, WorkloadType: "deployment"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload-configs", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetWorkloadConfig(t *testing.T) {
	id := uuid.New()
	wlSvc := &mockWorkloadConfigService{
		getFunc: func(ctx context.Context, uid uuid.UUID) (*domain.WorkloadConfig, error) {
			return &domain.WorkloadConfig{BaseModel: domain.BaseModel{ID: uid}, Name: "test"}, nil
		},
	}
	h := NewHandler(wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload-configs/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetWorkloadConfigNotFound(t *testing.T) {
	id := uuid.New()
	wlSvc := &mockWorkloadConfigService{
		getFunc: func(ctx context.Context, uid uuid.UUID) (*domain.WorkloadConfig, error) {
			return nil, sql.ErrNoRows
		},
	}
	h := NewHandler(wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload-configs/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListWorkloadConfigs(t *testing.T) {
	wlSvc := &mockWorkloadConfigService{
		listFunc: func(ctx context.Context, filter workloadconfig.WorkloadConfigListFilter) ([]domain.WorkloadConfig, error) {
			return []domain.WorkloadConfig{{BaseModel: domain.BaseModel{ID: uuid.New()}, Name: "wl1"}}, nil
		},
	}
	h := NewHandler(wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload-configs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateWorkloadConfig(t *testing.T) {
	id := uuid.New()
	wlSvc := &mockWorkloadConfigService{
		updateFunc: func(ctx context.Context, item *domain.WorkloadConfig) error {
			return nil
		},
	}
	h := NewHandler(wlSvc)
	r := setupTestRouter(h)

	reqBody, _ := json.Marshal(domain.WorkloadConfigInput{ApplicationID: uuid.New(), Name: "updated", Replicas: 2, WorkloadType: "deployment"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workload-configs/"+id.String(), bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

