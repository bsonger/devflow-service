package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	imageservice "github.com/bsonger/devflow-service/internal/image/service"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubImageService struct {
	createFn func(context.Context, *imagedomain.Image) (uuid.UUID, error)
	listFn   func(context.Context, imageservice.ImageListFilter) ([]imagedomain.Image, error)
	getFn    func(context.Context, uuid.UUID) (*imagedomain.Image, error)
	patchFn  func(context.Context, uuid.UUID, *imagedomain.PatchImageRequest) error
}

func (s stubImageService) CreateImage(ctx context.Context, m *imagedomain.Image) (uuid.UUID, error) {
	return s.createFn(ctx, m)
}

func (s stubImageService) List(ctx context.Context, filter imageservice.ImageListFilter) ([]imagedomain.Image, error) {
	return s.listFn(ctx, filter)
}

func (s stubImageService) Get(ctx context.Context, id uuid.UUID) (*imagedomain.Image, error) {
	return s.getFn(ctx, id)
}

func (s stubImageService) Patch(ctx context.Context, id uuid.UUID, patch *imagedomain.PatchImageRequest) error {
	return s.patchFn(ctx, id, patch)
}

func TestCreateImageReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ImageHandler{
		svc: stubImageService{
			createFn: func(_ context.Context, m *imagedomain.Image) (uuid.UUID, error) {
				m.WithCreateDefault()
				m.Name = "image-main-1"
				return m.GetID(), nil
			},
		},
	}

	r := gin.New()
	r.POST("/api/v1/images", handler.Create)

	body := bytes.NewBufferString(`{"application_id":"11111111-1111-1111-1111-111111111111","branch":"main"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/images", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d want %d", rec.Code, http.StatusCreated)
	}
	var payload struct {
		Data imagedomain.Image `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Data.ApplicationID == uuid.Nil || payload.Data.Branch != "main" {
		t.Fatalf("unexpected payload: %#v", payload.Data)
	}
}

func TestListImagesReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	id := uuid.New()
	handler := &ImageHandler{
		svc: stubImageService{
			listFn: func(_ context.Context, _ imageservice.ImageListFilter) ([]imagedomain.Image, error) {
				return []imagedomain.Image{{BaseModel: model.BaseModel{ID: id}, Name: "image-1", Branch: "main"}}, nil
			},
		},
	}

	r := gin.New()
	r.GET("/api/v1/images", handler.List)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/images", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want %d", rec.Code, http.StatusOK)
	}
	var payload struct {
		Data []imagedomain.Image `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].Name != "image-1" {
		t.Fatalf("unexpected payload: %#v", payload.Data)
	}
}

func TestGetImageReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	id := uuid.New()
	handler := &ImageHandler{
		svc: stubImageService{
			getFn: func(_ context.Context, got uuid.UUID) (*imagedomain.Image, error) {
				if got != id {
					t.Fatalf("unexpected id %s", got)
				}
				return &imagedomain.Image{BaseModel: model.BaseModel{ID: id}, Name: "image-1"}, nil
			},
		},
	}

	r := gin.New()
	r.GET("/api/v1/images/:id", handler.Get)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/images/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want %d", rec.Code, http.StatusOK)
	}
}

func TestPatchImageNotFoundReturnsErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ImageHandler{
		svc: stubImageService{
			patchFn: func(_ context.Context, _ uuid.UUID, _ *imagedomain.PatchImageRequest) error {
				return sql.ErrNoRows
			},
		},
	}
	r := gin.New()
	r.PATCH("/api/v1/images/:id", handler.Patch)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/images/"+uuid.New().String(), bytes.NewBufferString(`{"digest":"sha256:1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNotFound)
	}
}
