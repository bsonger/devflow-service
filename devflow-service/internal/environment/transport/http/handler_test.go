package http

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

	envsvc "github.com/bsonger/devflow-service/internal/environment/application"
	envdomain "github.com/bsonger/devflow-service/internal/environment/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubEnvironmentService struct {
	createFn func(context.Context, *envdomain.Environment) (uuid.UUID, error)
	getFn    func(context.Context, uuid.UUID) (*envdomain.Environment, error)
	updateFn func(context.Context, *envdomain.Environment) error
	deleteFn func(context.Context, uuid.UUID) error
	listFn   func(context.Context, envsvc.ListFilter) ([]envdomain.Environment, error)
}

func (s stubEnvironmentService) Create(ctx context.Context, environment *envdomain.Environment) (uuid.UUID, error) {
	return s.createFn(ctx, environment)
}
func (s stubEnvironmentService) Get(ctx context.Context, id uuid.UUID) (*envdomain.Environment, error) {
	return s.getFn(ctx, id)
}
func (s stubEnvironmentService) Update(ctx context.Context, environment *envdomain.Environment) error {
	return s.updateFn(ctx, environment)
}
func (s stubEnvironmentService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.deleteFn(ctx, id)
}
func (s stubEnvironmentService) List(ctx context.Context, filter envsvc.ListFilter) ([]envdomain.Environment, error) {
	return s.listFn(ctx, filter)
}

func TestCreateEnvironmentReturnsEnvelopeWithoutNamespace(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := NewHandler(stubEnvironmentService{createFn: func(_ context.Context, environment *envdomain.Environment) (uuid.UUID, error) {
		return environment.GetID(), nil
	}})

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
		Data envdomain.Environment `json:"data"`
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
	handler := NewHandler(stubEnvironmentService{createFn: func(_ context.Context, _ *envdomain.Environment) (uuid.UUID, error) {
		return uuid.Nil, envsvc.ErrClusterReferenceNotFound
	}})

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
	handler := NewHandler(stubEnvironmentService{})

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
	handler := NewHandler(stubEnvironmentService{deleteFn: func(_ context.Context, _ uuid.UUID) error {
		return sql.ErrNoRows
	}})

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
	handler := NewHandler(stubEnvironmentService{listFn: func(_ context.Context, _ envsvc.ListFilter) ([]envdomain.Environment, error) {
		return nil, nil
	}})

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
