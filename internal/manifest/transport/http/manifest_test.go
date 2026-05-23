package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	manifestservice "github.com/bsonger/devflow-service/internal/manifest/service"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubManifestService struct {
	createFn       func(context.Context, *manifestdomain.CreateManifestRequest) (*manifestdomain.Manifest, error)
	listFn         func(context.Context, manifestdomain.ManifestListFilter) ([]manifestdomain.Manifest, error)
	getFn          func(context.Context, uuid.UUID) (*manifestdomain.Manifest, error)
	getResourcesFn func(context.Context, uuid.UUID) (*manifestdomain.ManifestResourcesView, error)
	deleteFn       func(context.Context, uuid.UUID) error
}

func (s stubManifestService) CreateManifest(ctx context.Context, req *manifestdomain.CreateManifestRequest) (*manifestdomain.Manifest, error) {
	return s.createFn(ctx, req)
}

func (s stubManifestService) List(ctx context.Context, filter manifestdomain.ManifestListFilter) ([]manifestdomain.Manifest, error) {
	return s.listFn(ctx, filter)
}

func (s stubManifestService) Get(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
	return s.getFn(ctx, id)
}

func (s stubManifestService) GetResources(ctx context.Context, id uuid.UUID) (*manifestdomain.ManifestResourcesView, error) {
	return s.getResourcesFn(ctx, id)
}

func (s stubManifestService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.deleteFn(ctx, id)
}

func TestCreateManifestReturnsCreated(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ManifestHandler{
		svc: stubManifestService{
			createFn: func(_ context.Context, req *manifestdomain.CreateManifestRequest) (*manifestdomain.Manifest, error) {
				item := &manifestdomain.Manifest{ApplicationID: req.ApplicationID, GitRevision: "main", RepoAddress: "git@github.com:example/demo.git", CommitHash: "abcdef123456", ImageRef: "repo/demo@sha256:abc", ImageDigest: "sha256:abc", PipelineID: "pipe-1", TraceID: "trace-1", SpanID: "span-1", Status: model.ManifestPending}
				item.WithCreateDefault()
				return item, nil
			},
		},
	}
	r := gin.New()
	r.POST("/api/v1/manifests", handler.Create)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manifests", bytes.NewBufferString(`{"application_id":"11111111-1111-1111-1111-111111111111"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var payload struct {
		Data manifestdomain.Manifest `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Data.ImageRef == "" || payload.Data.CommitHash == "" || payload.Data.PipelineID == "" || payload.Data.GitRevision != "main" {
		t.Fatalf("unexpected payload %+v", payload.Data)
	}
	if payload.Data.Status != model.ManifestPending {
		t.Fatalf("status = %q, want %q", payload.Data.Status, model.ManifestPending)
	}
}

func TestCreateManifestReturnsEnvironmentAgnosticImageRef(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ManifestHandler{
		svc: stubManifestService{
			createFn: func(_ context.Context, req *manifestdomain.CreateManifestRequest) (*manifestdomain.Manifest, error) {
				item := &manifestdomain.Manifest{
					ApplicationID: req.ApplicationID,
					ImageRef:      "repo/demo@sha256:abc",
					Status:        model.ManifestAvailable,
				}
				item.WithCreateDefault()
				return item, nil
			},
		},
	}
	r := gin.New()
	r.POST("/api/v1/manifests", handler.Create)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manifests", bytes.NewBufferString(`{"application_id":"11111111-1111-1111-1111-111111111111"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var payload struct {
		Data manifestdomain.Manifest `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Data.ImageRef == "" {
		t.Fatalf("ImageRef = %q, want populated", payload.Data.ImageRef)
	}
}

func TestCreateManifestAcceptsGitRevision(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ManifestHandler{
		svc: stubManifestService{
			createFn: func(_ context.Context, req *manifestdomain.CreateManifestRequest) (*manifestdomain.Manifest, error) {
				if req.GitRevision != "main" {
					t.Fatalf("GitRevision = %q, want main", req.GitRevision)
				}
				item := &manifestdomain.Manifest{ApplicationID: req.ApplicationID, GitRevision: req.GitRevision, ImageRef: "repo/demo:tag", Status: model.ManifestAvailable}
				item.WithCreateDefault()
				return item, nil
			},
		},
	}
	r := gin.New()
	r.POST("/api/v1/manifests", handler.Create)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manifests", bytes.NewBufferString(`{"application_id":"11111111-1111-1111-1111-111111111111","git_revision":"main"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var payload struct {
		Data manifestdomain.Manifest `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Data.GitRevision != "main" {
		t.Fatalf("GitRevision = %q want main", payload.Data.GitRevision)
	}
}

func TestCreateManifestOmitsGitRevisionWhenNotProvided(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ManifestHandler{
		svc: stubManifestService{
			createFn: func(_ context.Context, req *manifestdomain.CreateManifestRequest) (*manifestdomain.Manifest, error) {
				if req.GitRevision != "" {
					t.Fatalf("GitRevision = %q, want empty before service defaulting", req.GitRevision)
				}
				item := &manifestdomain.Manifest{ApplicationID: req.ApplicationID, Status: model.ManifestAvailable}
				item.WithCreateDefault()
				return item, nil
			},
		},
	}
	r := gin.New()
	r.POST("/api/v1/manifests", handler.Create)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manifests", bytes.NewBufferString(`{"application_id":"11111111-1111-1111-1111-111111111111"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestGetManifestNotFound(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ManifestHandler{
		svc: stubManifestService{
			getFn: func(_ context.Context, _ uuid.UUID) (*manifestdomain.Manifest, error) { return nil, sql.ErrNoRows },
		},
	}
	r := gin.New()
	r.GET("/api/v1/manifests/:id", handler.Get)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/manifests/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetManifestResourcesReturnsGroupedFrozenObjects(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	manifestID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	handler := &ManifestHandler{
		svc: stubManifestService{
			getResourcesFn: func(_ context.Context, id uuid.UUID) (*manifestdomain.ManifestResourcesView, error) {
				if id != manifestID {
					t.Fatalf("id = %s want %s", id, manifestID)
				}
				return &manifestdomain.ManifestResourcesView{
					ManifestID:    manifestID,
					ApplicationID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
					Resources: manifestdomain.ManifestGroupedResources{
						ConfigMap: &manifestdomain.ManifestRenderedResource{
							Kind:      "ConfigMap",
							Name:      "demo-api-config",
							Namespace: "staging",
							YAML:      "apiVersion: v1",
							Object:    map[string]any{"kind": "ConfigMap"},
						},
						Deployment: &manifestdomain.ManifestRenderedResource{
							Kind:      "Deployment",
							Name:      "demo-api",
							Namespace: "staging",
							YAML:      "apiVersion: apps/v1",
							Object:    map[string]any{"kind": "Deployment"},
						},
						Services: []manifestdomain.ManifestRenderedResource{{
							Kind:      "Service",
							Name:      "demo-api",
							Namespace: "staging",
							YAML:      "apiVersion: v1",
							Object:    map[string]any{"kind": "Service"},
						}},
					},
				}, nil
			},
		},
	}
	r := gin.New()
	r.GET("/api/v1/manifests/:id/resources", handler.GetResources)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/manifests/"+manifestID.String()+"/resources", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload struct {
		Data manifestdomain.ManifestResourcesView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Data.Resources.ConfigMap == nil || payload.Data.Resources.Deployment == nil {
		t.Fatalf("unexpected grouped resources %+v", payload.Data.Resources)
	}
	if payload.Data.Resources.Rollout != nil {
		t.Fatalf("expected rollout nil, got %+v", payload.Data.Resources.Rollout)
	}
	if len(payload.Data.Resources.Services) != 1 {
		t.Fatalf("services len = %d want 1", len(payload.Data.Resources.Services))
	}
}

func TestCreateManifestMissingWorkloadReturns409(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ManifestHandler{
		svc: stubManifestService{
			createFn: func(_ context.Context, _ *manifestdomain.CreateManifestRequest) (*manifestdomain.Manifest, error) {
				return nil, manifestservice.ErrManifestWorkloadConfigMissing
			},
		},
	}

	r := gin.New()
	r.POST("/api/v1/manifests", handler.Create)

	body := bytes.NewBufferString(`{"application_id":"11111111-1111-1111-1111-111111111111"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manifests", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d want %d for workload missing", rec.Code, http.StatusConflict)
	}
	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if resp.Error.Code != "failed_precondition" {
		t.Fatalf("error code = %q, want failed_precondition", resp.Error.Code)
	}
}

func TestDeleteManifestReturns204(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	manifestID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	handler := &ManifestHandler{
		svc: stubManifestService{
			deleteFn: func(_ context.Context, id uuid.UUID) error {
				if id != manifestID {
					t.Fatalf("id = %s want %s", id, manifestID)
				}
				return nil
			},
		},
	}
	r := gin.New()
	r.DELETE("/api/v1/manifests/:id", handler.Delete)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manifests/"+manifestID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

func TestDeleteManifestNotFoundReturns404(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ManifestHandler{
		svc: stubManifestService{
			deleteFn: func(_ context.Context, _ uuid.UUID) error {
				return sql.ErrNoRows
			},
		},
	}
	r := gin.New()
	r.DELETE("/api/v1/manifests/:id", handler.Delete)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manifests/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDeleteManifestInvalidIDReturns400(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ManifestHandler{
		svc: stubManifestService{
			deleteFn: func(_ context.Context, _ uuid.UUID) error {
				t.Fatal("deleteFn should not be called for invalid id")
				return nil
			},
		},
	}
	r := gin.New()
	r.DELETE("/api/v1/manifests/:id", handler.Delete)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manifests/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want %d", rec.Code, http.StatusBadRequest)
	}
}
