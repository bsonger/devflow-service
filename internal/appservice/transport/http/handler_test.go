package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsonger/devflow-service/internal/appservice/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type mockServiceService struct {
	createFunc func(ctx context.Context, service *domain.Service) (uuid.UUID, error)
	getFunc    func(ctx context.Context, applicationId, id uuid.UUID) (*domain.Service, error)
	updateFunc func(ctx context.Context, service *domain.Service) error
	deleteFunc func(ctx context.Context, applicationId, id uuid.UUID) error
	listFunc   func(ctx context.Context, filter ServiceListFilter) ([]domain.Service, error)
}

func (m *mockServiceService) Create(ctx context.Context, service *domain.Service) (uuid.UUID, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, service)
	}
	return uuid.New(), nil
}

func (m *mockServiceService) Get(ctx context.Context, applicationId, id uuid.UUID) (*domain.Service, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, applicationId, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockServiceService) Update(ctx context.Context, service *domain.Service) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, service)
	}
	return nil
}

func (m *mockServiceService) Delete(ctx context.Context, applicationId, id uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, applicationId, id)
	}
	return nil
}

func (m *mockServiceService) List(ctx context.Context, filter ServiceListFilter) ([]domain.Service, error) {
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

func TestCreateService(t *testing.T) {
	svc := &mockServiceService{}
	h := NewHandler(svc)
	r := setupTestRouter(h)

	appID := uuid.New()
	reqBody, _ := json.Marshal(domain.ServiceInput{ApplicationID: appID, Name: "test-svc", Ports: []domain.ServicePort{{ServicePort: 80}}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateServiceInvalidApplicationID(t *testing.T) {
	svc := &mockServiceService{}
	h := NewHandler(svc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services", bytes.NewBufferString(`{"application_id":"invalid-id","name":"test-svc","ports":[{"service_port":80}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListServices(t *testing.T) {
	appID := uuid.New()
	svc := &mockServiceService{
		listFunc: func(ctx context.Context, filter ServiceListFilter) ([]domain.Service, error) {
			return []domain.Service{{BaseModel: domain.BaseModel{ID: uuid.New()}, ApplicationID: appID, Name: "svc1"}}, nil
		},
	}
	h := NewHandler(svc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services?application_id="+appID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateService(t *testing.T) {
	appID := uuid.New()
	svcID := uuid.New()
	svc := &mockServiceService{
		updateFunc: func(ctx context.Context, service *domain.Service) error {
			return nil
		},
	}
	h := NewHandler(svc)
	r := setupTestRouter(h)

	reqBody, _ := json.Marshal(domain.ServiceInput{ApplicationID: appID, Name: "updated-svc", Ports: []domain.ServicePort{{ServicePort: 8080}}})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/services/"+svcID.String(), bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteService(t *testing.T) {
	appID := uuid.New()
	svcID := uuid.New()
	svc := &mockServiceService{
		deleteFunc: func(ctx context.Context, applicationId, id uuid.UUID) error {
			return nil
		},
	}
	h := NewHandler(svc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/services/"+svcID.String()+"?application_id="+appID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}
