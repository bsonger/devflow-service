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

	"github.com/bsonger/devflow-service/internal/approute/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type mockRouteService struct {
	createFunc   func(ctx context.Context, route *domain.Route) (uuid.UUID, error)
	getFunc      func(ctx context.Context, applicationID, id uuid.UUID) (*domain.Route, error)
	updateFunc   func(ctx context.Context, route *domain.Route) error
	deleteFunc   func(ctx context.Context, applicationID, id uuid.UUID) error
	listFunc     func(ctx context.Context, filter RouteListFilter) ([]domain.Route, error)
	validateFunc func(ctx context.Context, route *domain.Route) []string
}

func (m *mockRouteService) Create(ctx context.Context, route *domain.Route) (uuid.UUID, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, route)
	}
	return uuid.New(), nil
}

func (m *mockRouteService) Get(ctx context.Context, applicationID, id uuid.UUID) (*domain.Route, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, applicationID, id)
	}
	return nil, sql.ErrNoRows
}

func (m *mockRouteService) Update(ctx context.Context, route *domain.Route) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, route)
	}
	return nil
}

func (m *mockRouteService) Delete(ctx context.Context, applicationID, id uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, applicationID, id)
	}
	return nil
}

func (m *mockRouteService) List(ctx context.Context, filter RouteListFilter) ([]domain.Route, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, filter)
	}
	return nil, nil
}

func (m *mockRouteService) Validate(ctx context.Context, route *domain.Route) []string {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, route)
	}
	return nil
}

func setupTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)
	return r
}

func TestCreateRoute(t *testing.T) {
	routeSvc := &mockRouteService{}
	h := NewHandler(routeSvc)
	r := setupTestRouter(h)

	appID := uuid.New()
	reqBody, _ := json.Marshal(domain.RouteInput{ApplicationID: appID, EnvironmentID: "staging", Name: "test-route", Host: "example.com", Path: "/api", ServiceName: "svc", ServicePort: 80})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListRoutes(t *testing.T) {
	appID := uuid.New()
	routeSvc := &mockRouteService{
		listFunc: func(ctx context.Context, filter RouteListFilter) ([]domain.Route, error) {
			return []domain.Route{{BaseModel: domain.BaseModel{ID: uuid.New()}, ApplicationID: appID, Name: "route1"}}, nil
		},
	}
	h := NewHandler(routeSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes?application_id="+appID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestValidateRoute(t *testing.T) {
	appID := uuid.New()
	routeSvc := &mockRouteService{
		validateFunc: func(ctx context.Context, route *domain.Route) []string {
			return nil
		},
	}
	h := NewHandler(routeSvc)
	r := setupTestRouter(h)

	reqBody, _ := json.Marshal(domain.RouteInput{ApplicationID: appID, EnvironmentID: "staging", Name: "test-route", Host: "example.com", Path: "/api", ServiceName: "svc", ServicePort: 80})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes:validate", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data domain.RouteValidationResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Data.Valid {
		t.Fatalf("expected valid route")
	}
}

func TestValidateRouteInvalid(t *testing.T) {
	appID := uuid.New()
	routeSvc := &mockRouteService{
		validateFunc: func(ctx context.Context, route *domain.Route) []string {
			return []string{"service_name does not exist"}
		},
	}
	h := NewHandler(routeSvc)
	r := setupTestRouter(h)

	reqBody, _ := json.Marshal(domain.RouteInput{ApplicationID: appID, EnvironmentID: "staging", Name: "test-route", Host: "example.com", Path: "/api", ServiceName: "missing", ServicePort: 80})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes:validate", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data domain.RouteValidationResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Data.Valid {
		t.Fatalf("expected invalid route")
	}
	if len(resp.Data.Errors) != 1 || resp.Data.Errors[0] != "service_name does not exist" {
		t.Fatalf("unexpected errors: %v", resp.Data.Errors)
	}
}

func TestDeleteRouteNotFound(t *testing.T) {
	appID := uuid.New()
	routeID := uuid.New()
	routeSvc := &mockRouteService{
		deleteFunc: func(ctx context.Context, applicationID, id uuid.UUID) error {
			return sql.ErrNoRows
		},
	}
	h := NewHandler(routeSvc)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/routes/"+routeID.String()+"?application_id="+appID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCreateRouteValidationError(t *testing.T) {
	routeSvc := &mockRouteService{
		createFunc: func(ctx context.Context, route *domain.Route) (uuid.UUID, error) {
			return uuid.Nil, errors.New("name is required")
		},
	}
	h := NewHandler(routeSvc)
	r := setupTestRouter(h)

	reqBody, _ := json.Marshal(domain.RouteInput{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
