package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsonger/devflow-service/internal/appconfig/domain"
	appconfig "github.com/bsonger/devflow-service/internal/appconfig/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type mockAppConfigService struct {
	createFunc func(ctx context.Context, cfg *domain.AppConfig) (uuid.UUID, error)
	getFunc    func(ctx context.Context, id uuid.UUID) (*domain.AppConfig, error)
	updateFunc func(ctx context.Context, cfg *domain.AppConfig) error
	deleteFunc func(ctx context.Context, id uuid.UUID) error
	listFunc   func(ctx context.Context, filter appconfig.AppConfigListFilter) ([]domain.AppConfig, error)
	syncFunc   func(ctx context.Context, id uuid.UUID) (*appconfig.AppConfigSyncResult, error)
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

func (m *mockAppConfigService) List(ctx context.Context, filter appconfig.AppConfigListFilter) ([]domain.AppConfig, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, filter)
	}
	return nil, nil
}

func (m *mockAppConfigService) Sync(ctx context.Context, id uuid.UUID) (*appconfig.AppConfigSyncResult, error) {
	if m.syncFunc != nil {
		return m.syncFunc(ctx, id)
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
	h := NewHandler(appSvc)
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
	h := NewHandler(appSvc)
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
	h := NewHandler(appSvc)
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
		listFunc: func(ctx context.Context, filter appconfig.AppConfigListFilter) ([]domain.AppConfig, error) {
			return []domain.AppConfig{{BaseModel: domain.BaseModel{ID: uuid.New()}, Name: "cfg1"}}, nil
		},
	}
	h := NewHandler(appSvc)
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
		syncFunc: func(ctx context.Context, uid uuid.UUID) (*appconfig.AppConfigSyncResult, error) {
			return &appconfig.AppConfigSyncResult{Revision: &domain.AppConfigRevision{ID: uuid.New()}, Created: true}, nil
		},
	}
	h := NewHandler(appSvc)
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
		syncFunc: func(ctx context.Context, uid uuid.UUID) (*appconfig.AppConfigSyncResult, error) {
			return nil, appconfig.ErrConfigRepositoryUnavailable
		},
	}
	h := NewHandler(appSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/app-configs/"+id.String()+"/sync-from-repo", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFailedDependency {
		t.Fatalf("expected 424, got %d", rec.Code)
	}
}
