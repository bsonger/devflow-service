package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsonger/devflow-service/internal/config/domain"
	"github.com/bsonger/devflow-service/internal/config/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type mockAppConfigService struct {
	createFunc func(ctx context.Context, cfg *domain.AppConfig) (uuid.UUID, error)
	getFunc    func(ctx context.Context, id uuid.UUID) (*domain.AppConfig, error)
	updateFunc func(ctx context.Context, cfg *domain.AppConfig) error
	deleteFunc func(ctx context.Context, id uuid.UUID) error
	listFunc   func(ctx context.Context, filter service.AppConfigListFilter) ([]domain.AppConfig, error)
	syncFunc   func(ctx context.Context, id uuid.UUID) (*service.AppConfigSyncResult, error)
}

func (m *mockAppConfigService) Create(ctx context.Context, cfg *domain.AppConfig) (uuid.UUID, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, cfg)
	}
	return uuid.New(), nil
}

func (m *mockAppConfigService) Get(ctx context.Context, id uuid.UUID) (*domain.AppConfig, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockAppConfigService) Update(ctx context.Context, cfg *domain.AppConfig) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, cfg)
	}
	return nil
}

func (m *mockAppConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *mockAppConfigService) List(ctx context.Context, filter service.AppConfigListFilter) ([]domain.AppConfig, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, filter)
	}
	return nil, nil
}

func (m *mockAppConfigService) Sync(ctx context.Context, id uuid.UUID) (*service.AppConfigSyncResult, error) {
	if m.syncFunc != nil {
		return m.syncFunc(ctx, id)
	}
	return nil, nil
}

type mockWorkloadConfigService struct {
	createFunc func(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error)
	getFunc    func(ctx context.Context, id uuid.UUID) (*domain.WorkloadConfig, error)
	updateFunc func(ctx context.Context, item *domain.WorkloadConfig) error
	deleteFunc func(ctx context.Context, id uuid.UUID) error
	listFunc   func(ctx context.Context, filter service.WorkloadConfigListFilter) ([]domain.WorkloadConfig, error)
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

func (m *mockWorkloadConfigService) List(ctx context.Context, filter service.WorkloadConfigListFilter) ([]domain.WorkloadConfig, error) {
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

func TestCreateAppConfig(t *testing.T) {
	appSvc := &mockAppConfigService{}
	wlSvc := &mockWorkloadConfigService{}
	h := NewHandler(appSvc, wlSvc)
	r := setupTestRouter(h)

	reqBody, _ := json.Marshal(domain.AppConfigInput{ApplicationID: uuid.New(), EnvironmentID: "staging", Name: "test-config"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app-configs", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetAppConfig(t *testing.T) {
	id := uuid.New()
	appSvc := &mockAppConfigService{
		getFunc: func(ctx context.Context, uid uuid.UUID) (*domain.AppConfig, error) {
			return &domain.AppConfig{BaseModel: domain.BaseModel{ID: uid}, Name: "test"}, nil
		},
	}
	wlSvc := &mockWorkloadConfigService{}
	h := NewHandler(appSvc, wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app-configs/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetAppConfigNotFound(t *testing.T) {
	id := uuid.New()
	appSvc := &mockAppConfigService{
		getFunc: func(ctx context.Context, uid uuid.UUID) (*domain.AppConfig, error) {
			return nil, sql.ErrNoRows
		},
	}
	wlSvc := &mockWorkloadConfigService{}
	h := NewHandler(appSvc, wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app-configs/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListAppConfigs(t *testing.T) {
	appSvc := &mockAppConfigService{
		listFunc: func(ctx context.Context, filter service.AppConfigListFilter) ([]domain.AppConfig, error) {
			return []domain.AppConfig{{BaseModel: domain.BaseModel{ID: uuid.New()}, Name: "cfg1"}}, nil
		},
	}
	wlSvc := &mockWorkloadConfigService{}
	h := NewHandler(appSvc, wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app-configs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSyncAppConfig(t *testing.T) {
	id := uuid.New()
	appSvc := &mockAppConfigService{
		syncFunc: func(ctx context.Context, uid uuid.UUID) (*service.AppConfigSyncResult, error) {
			return &service.AppConfigSyncResult{Revision: &domain.AppConfigRevision{ID: uuid.New()}, Created: true}, nil
		},
	}
	wlSvc := &mockWorkloadConfigService{}
	h := NewHandler(appSvc, wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/app-configs/"+id.String()+"/sync-from-repo", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSyncAppConfigUnavailable(t *testing.T) {
	id := uuid.New()
	appSvc := &mockAppConfigService{
		syncFunc: func(ctx context.Context, uid uuid.UUID) (*service.AppConfigSyncResult, error) {
			return nil, service.ErrConfigRepositoryUnavailable
		},
	}
	wlSvc := &mockWorkloadConfigService{}
	h := NewHandler(appSvc, wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/app-configs/"+id.String()+"/sync-from-repo", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFailedDependency {
		t.Fatalf("expected 424, got %d", rec.Code)
	}
}

func TestCreateWorkloadConfig(t *testing.T) {
	appSvc := &mockAppConfigService{}
	wlSvc := &mockWorkloadConfigService{}
	h := NewHandler(appSvc, wlSvc)
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
	appSvc := &mockAppConfigService{}
	wlSvc := &mockWorkloadConfigService{
		getFunc: func(ctx context.Context, uid uuid.UUID) (*domain.WorkloadConfig, error) {
			return &domain.WorkloadConfig{BaseModel: domain.BaseModel{ID: uid}, Name: "test"}, nil
		},
	}
	h := NewHandler(appSvc, wlSvc)
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
	appSvc := &mockAppConfigService{}
	wlSvc := &mockWorkloadConfigService{
		getFunc: func(ctx context.Context, uid uuid.UUID) (*domain.WorkloadConfig, error) {
			return nil, sql.ErrNoRows
		},
	}
	h := NewHandler(appSvc, wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload-configs/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListWorkloadConfigs(t *testing.T) {
	appSvc := &mockAppConfigService{}
	wlSvc := &mockWorkloadConfigService{
		listFunc: func(ctx context.Context, filter service.WorkloadConfigListFilter) ([]domain.WorkloadConfig, error) {
			return []domain.WorkloadConfig{{BaseModel: domain.BaseModel{ID: uuid.New()}, Name: "wl1"}}, nil
		},
	}
	h := NewHandler(appSvc, wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload-configs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteAppConfig(t *testing.T) {
	id := uuid.New()
	appSvc := &mockAppConfigService{
		deleteFunc: func(ctx context.Context, uid uuid.UUID) error {
			return nil
		},
	}
	wlSvc := &mockWorkloadConfigService{}
	h := NewHandler(appSvc, wlSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/app-configs/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestUpdateWorkloadConfig(t *testing.T) {
	id := uuid.New()
	appSvc := &mockAppConfigService{}
	wlSvc := &mockWorkloadConfigService{
		updateFunc: func(ctx context.Context, item *domain.WorkloadConfig) error {
			return nil
		},
	}
	h := NewHandler(appSvc, wlSvc)
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

func TestCreateAppConfigValidationError(t *testing.T) {
	appSvc := &mockAppConfigService{
		createFunc: func(ctx context.Context, cfg *domain.AppConfig) (uuid.UUID, error) {
			return uuid.Nil, errors.New("name is required")
		},
	}
	wlSvc := &mockWorkloadConfigService{}
	h := NewHandler(appSvc, wlSvc)
	r := setupTestRouter(h)

	reqBody, _ := json.Marshal(domain.AppConfigInput{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app-configs", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
