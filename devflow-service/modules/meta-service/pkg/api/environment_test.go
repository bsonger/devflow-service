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

	"github.com/bsonger/devflow-service/modules/meta-service/pkg/app"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubEnvironmentService struct {
	createFn func(context.Context, *domain.Environment) (uuid.UUID, error)
	getFn    func(context.Context, uuid.UUID) (*domain.Environment, error)
	updateFn func(context.Context, *domain.Environment) error
	deleteFn func(context.Context, uuid.UUID) error
	listFn   func(context.Context, app.EnvironmentListFilter) ([]domain.Environment, error)
}

func (s stubEnvironmentService) Create(ctx context.Context, environment *domain.Environment) (uuid.UUID, error) {
	return s.createFn(ctx, environment)
}
func (s stubEnvironmentService) Get(ctx context.Context, id uuid.UUID) (*domain.Environment, error) {
	return s.getFn(ctx, id)
}
func (s stubEnvironmentService) Update(ctx context.Context, environment *domain.Environment) error {
	return s.updateFn(ctx, environment)
}
func (s stubEnvironmentService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.deleteFn(ctx, id)
}
func (s stubEnvironmentService) List(ctx context.Context, filter app.EnvironmentListFilter) ([]domain.Environment, error) {
	return s.listFn(ctx, filter)
}

func TestCreateEnvironmentReturnsEnvelopeWithoutNamespace(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &EnvironmentHandler{svc: stubEnvironmentService{createFn: func(_ context.Context, environment *domain.Environment) (uuid.UUID, error) {
		return environment.GetID(), nil
	}}}

	r := gin.New()
	r.POST("/api/v1/environments", handler.Create)

	body := bytes.NewBufferString(`{"name":"staging","cluster_id":"11111111-1111-1111-1111-111111111111","description":"pre-prod","labels":[{"key":"tier","value":"staging"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d want %d", rec.Code, http.StatusCreated)
	}
	if strings.Contains(rec.Body.String(), "namespace") {
		t.Fatalf("environment payload should not contain namespace: %s", rec.Body.String())
	}

	var payload struct {
		Data domain.Environment `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Data.Name != "staging" || payload.Data.ClusterID.String() != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected payload: %#v", payload.Data)
	}
}

func TestCreateEnvironmentMissingClusterReferenceReturnsInvalidArgument(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &EnvironmentHandler{svc: stubEnvironmentService{createFn: func(_ context.Context, _ *domain.Environment) (uuid.UUID, error) {
		return uuid.Nil, app.ErrClusterReferenceNotFound
	}}}

	r := gin.New()
	r.POST("/api/v1/environments", handler.Create)

	body := bytes.NewBufferString(`{"name":"staging","cluster_id":"11111111-1111-1111-1111-111111111111"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_argument") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestGetEnvironmentInvalidIDReturnsInvalidArgument(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &EnvironmentHandler{svc: stubEnvironmentService{}}

	r := gin.New()
	r.GET("/api/v1/environments/:id", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/not-a-uuid", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDeleteEnvironmentNotFoundReturnsErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &EnvironmentHandler{svc: stubEnvironmentService{deleteFn: func(_ context.Context, _ uuid.UUID) error {
		return sql.ErrNoRows
	}}}

	r := gin.New()
	r.DELETE("/api/v1/environments/:id", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/environments/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNotFound)
	}
}

func TestListEnvironmentsRejectsInvalidClusterIDFilter(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &EnvironmentHandler{svc: stubEnvironmentService{listFn: func(_ context.Context, _ app.EnvironmentListFilter) ([]domain.Environment, error) {
		return nil, nil
	}}}

	r := gin.New()
	r.GET("/api/v1/environments", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments?cluster_id=bad&page=1&page_size=20", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWriteEnvironmentErrorFallsBackToInternal(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/boom", func(c *gin.Context) { writeEnvironmentError(c, errors.New("boom")) })

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d want %d", rec.Code, http.StatusInternalServerError)
	}
}
