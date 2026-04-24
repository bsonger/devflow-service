package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appsvc "github.com/bsonger/devflow-service/internal/application/application"
	appdomain "github.com/bsonger/devflow-service/internal/application/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubApplicationService struct {
	createFn            func(context.Context, *appdomain.Application) (uuid.UUID, error)
	getFn               func(context.Context, uuid.UUID) (*appdomain.Application, error)
	updateFn            func(context.Context, *appdomain.Application) error
	deleteFn            func(context.Context, uuid.UUID) error
	updateActiveImageFn func(context.Context, uuid.UUID, uuid.UUID) error
	listFn              func(context.Context, appsvc.ListFilter) ([]appdomain.Application, error)
}

func (s stubApplicationService) Create(ctx context.Context, app *appdomain.Application) (uuid.UUID, error) {
	return s.createFn(ctx, app)
}
func (s stubApplicationService) Get(ctx context.Context, id uuid.UUID) (*appdomain.Application, error) {
	return s.getFn(ctx, id)
}
func (s stubApplicationService) Update(ctx context.Context, app *appdomain.Application) error {
	return s.updateFn(ctx, app)
}
func (s stubApplicationService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.deleteFn(ctx, id)
}
func (s stubApplicationService) UpdateActiveImage(ctx context.Context, appID, imageID uuid.UUID) error {
	return s.updateActiveImageFn(ctx, appID, imageID)
}
func (s stubApplicationService) List(ctx context.Context, filter appsvc.ListFilter) ([]appdomain.Application, error) {
	return s.listFn(ctx, filter)
}

func TestCreateApplicationReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := NewHandler(stubApplicationService{createFn: func(_ context.Context, app *appdomain.Application) (uuid.UUID, error) { return app.GetID(), nil }})

	r := gin.New()
	r.POST("/api/v1/applications", handler.Create)

	body := bytes.NewBufferString(`{"project_id":"11111111-1111-1111-1111-111111111111","name":"web","repo_address":"git@github.com:bsonger/web.git","description":"customer web","labels":[{"key":"tier","value":"frontend"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d want %d", rec.Code, http.StatusCreated)
	}

	var payload struct {
		Data appdomain.Application `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Data.Name != "web" || payload.Data.Description != "customer web" || payload.Data.Labels[0].Value != "frontend" {
		t.Fatalf("unexpected payload: %#v", payload.Data)
	}
}

func TestListApplicationsReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := NewHandler(stubApplicationService{listFn: func(_ context.Context, filter appsvc.ListFilter) ([]appdomain.Application, error) {
		if filter.ProjectID != nil {
			t.Fatalf("unexpected project filter: %#v", filter)
		}
		return []appdomain.Application{{Name: "web"}}, nil
	}})

	r := gin.New()
	r.GET("/api/v1/applications", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications?page=1&page_size=20", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Data       []appdomain.Application `json:"data"`
		Pagination struct {
			Page     int `json:"page"`
			PageSize int `json:"page_size"`
			Total    int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(payload.Data) != 1 || payload.Pagination.Total != 1 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestUpdateActiveImageReturnsNoContent(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := NewHandler(stubApplicationService{updateActiveImageFn: func(_ context.Context, _, _ uuid.UUID) error { return nil }})

	r := gin.New()
	r.PATCH("/api/v1/applications/:id/active_image", handler.UpdateActiveImage)

	body := bytes.NewBufferString(`{"image_id":"22222222-2222-2222-2222-222222222222"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/applications/"+uuid.New().String()+"/active_image", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNoContent)
	}
}

func TestGetApplicationNotFoundReturnsErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := NewHandler(stubApplicationService{getFn: func(_ context.Context, _ uuid.UUID) (*appdomain.Application, error) { return nil, sql.ErrNoRows }})

	r := gin.New()
	r.GET("/api/v1/applications/:id", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNotFound)
	}
}
