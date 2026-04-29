package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/release/service"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubReleaseService struct {
	createFn    func(context.Context, *model.Release) (uuid.UUID, error)
	getFn       func(context.Context, uuid.UUID) (*model.Release, error)
	getBundleFn func(context.Context, uuid.UUID) (*model.ReleaseBundlePreview, error)
	listFn      func(context.Context, service.ReleaseListFilter) ([]*model.Release, error)
	deleteFn    func(context.Context, uuid.UUID) error
}

func (s stubReleaseService) Create(ctx context.Context, release *model.Release) (uuid.UUID, error) {
	return s.createFn(ctx, release)
}

func (s stubReleaseService) Get(ctx context.Context, id uuid.UUID) (*model.Release, error) {
	return s.getFn(ctx, id)
}

func (s stubReleaseService) GetBundlePreview(ctx context.Context, id uuid.UUID) (*model.ReleaseBundlePreview, error) {
	return s.getBundleFn(ctx, id)
}

func (s stubReleaseService) List(ctx context.Context, filter service.ReleaseListFilter) ([]*model.Release, error) {
	return s.listFn(ctx, filter)
}

func (s stubReleaseService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.deleteFn(ctx, id)
}

func TestCreateReleaseReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			createFn: func(_ context.Context, release *model.Release) (uuid.UUID, error) {
				release.WithCreateDefault()
				release.ManifestID = uuid.MustParse("33333333-3333-3333-3333-333333333333")
				release.ApplicationID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
				release.Strategy = "blueGreen"
				release.Status = model.ReleasePending
				return release.GetID(), nil
			},
		},
	}

	r := gin.New()
	r.POST("/api/v1/releases", handler.Create)

	body := bytes.NewBufferString(`{"manifest_id":"22222222-2222-2222-2222-222222222222","environment_id":"prod","strategy":"blueGreen","type":"upgrade"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/releases", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d want %d", rec.Code, http.StatusCreated)
	}

	var payload struct {
		Data model.Release `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Data.ManifestID == uuid.Nil || payload.Data.EnvironmentID != "prod" || payload.Data.Strategy != "blueGreen" {
		t.Fatalf("unexpected payload: %#v", payload.Data)
	}
}

func TestCreateReleaseFailedPreconditionReturnsErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			createFn: func(_ context.Context, _ *model.Release) (uuid.UUID, error) {
				return uuid.Nil, service.ErrReleaseManifestNotReady
			},
		},
	}

	r := gin.New()
	r.POST("/api/v1/releases", handler.Create)

	body := bytes.NewBufferString(`{"manifest_id":"22222222-2222-2222-2222-222222222222","environment_id":"prod","strategy":"rolling"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/releases", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d want %d", rec.Code, http.StatusConflict)
	}
}

func TestCreateReleaseClusterNotReadyReturns409(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			createFn: func(_ context.Context, _ *model.Release) (uuid.UUID, error) {
				return uuid.Nil, releasesupport.ErrDeployTargetClusterNotReady
			},
		},
	}

	r := gin.New()
	r.POST("/api/v1/releases", handler.Create)

	body := bytes.NewBufferString(`{"manifest_id":"22222222-2222-2222-2222-222222222222","environment_id":"prod","strategy":"rolling"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/releases", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d want %d for cluster not ready", rec.Code, http.StatusConflict)
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

func TestCreateReleaseClusterReadinessMalformedReturns409(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			createFn: func(_ context.Context, _ *model.Release) (uuid.UUID, error) {
				return uuid.Nil, releasesupport.ErrDeployTargetClusterReadinessMalformed
			},
		},
	}

	r := gin.New()
	r.POST("/api/v1/releases", handler.Create)

	body := bytes.NewBufferString(`{"manifest_id":"22222222-2222-2222-2222-222222222222","environment_id":"prod","strategy":"rolling"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/releases", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d want %d for readiness malformed", rec.Code, http.StatusConflict)
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
	if resp.Error.Message != releasesupport.ErrDeployTargetClusterReadinessMalformed.Error() {
		t.Fatalf("error message = %q, want %q", resp.Error.Message, releasesupport.ErrDeployTargetClusterReadinessMalformed.Error())
	}
}

func TestCreateReleaseClusterNotReadyDoesNotReturnInternal500(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			createFn: func(_ context.Context, _ *model.Release) (uuid.UUID, error) {
				return uuid.Nil, releasesupport.ErrDeployTargetClusterNotReady
			},
		},
	}

	r := gin.New()
	r.POST("/api/v1/releases", handler.Create)

	body := bytes.NewBufferString(`{"manifest_id":"22222222-2222-2222-2222-222222222222","environment_id":"prod","strategy":"rolling"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/releases", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code == http.StatusInternalServerError {
		t.Fatalf("readiness blocker must not surface as 500 internal, got %d", rec.Code)
	}
}

func TestGetReleaseReturnsBundleSummary(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			getFn: func(_ context.Context, id uuid.UUID) (*model.Release, error) {
				if id != releaseID {
					t.Fatalf("id = %s want %s", id, releaseID)
				}
				renderedAt := time.Now().UTC()
				publishedAt := renderedAt
				return &model.Release{
					BaseModel:     model.BaseModel{ID: releaseID},
					ApplicationID: uuid.New(),
					ManifestID:    uuid.New(),
					EnvironmentID: "staging",
					Strategy:      "canary",
					Type:          "Upgrade",
					Status:        model.ReleaseRunning,
					BundleSummary: &model.ReleaseBundleSummary{
						Available:           true,
						Namespace:           "checkout",
						ArtifactName:        "demo-api",
						BundleDigest:        "sha256:bundle",
						PrimaryWorkloadKind: "Rollout",
						ResourceCounts: model.ReleaseBundleResourceCounts{
							Services: 2,
							Rollouts: 1,
							Total:    3,
						},
						Artifact: &model.ReleaseBundleArtifact{
							Repository: "registry.example.com/devflow/releases/demo-api",
							Digest:     "sha256:artifact",
							Ref:        "oci://registry.example.com/devflow/releases/demo-api@sha256:artifact",
						},
						RenderedAt:  &renderedAt,
						PublishedAt: &publishedAt,
					},
				}, nil
			},
		},
	}

	r := gin.New()
	r.GET("/api/v1/releases/:id", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases/"+releaseID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Data struct {
			ID            string `json:"id"`
			BundleSummary struct {
				Available           bool   `json:"available"`
				Namespace           string `json:"namespace"`
				ArtifactName        string `json:"artifact_name"`
				BundleDigest        string `json:"bundle_digest"`
				PrimaryWorkloadKind string `json:"primary_workload_kind"`
				ResourceCounts      struct {
					Services int `json:"services"`
					Rollouts int `json:"rollouts"`
					Total    int `json:"total"`
				} `json:"resource_counts"`
				Artifact struct {
					Ref string `json:"ref"`
				} `json:"artifact"`
			} `json:"bundle_summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Data.BundleSummary.Namespace != "checkout" {
		t.Fatalf("bundle_summary.namespace = %q", payload.Data.BundleSummary.Namespace)
	}
	if payload.Data.BundleSummary.BundleDigest != "sha256:bundle" {
		t.Fatalf("bundle_summary.bundle_digest = %q", payload.Data.BundleSummary.BundleDigest)
	}
	if payload.Data.BundleSummary.PrimaryWorkloadKind != "Rollout" {
		t.Fatalf("bundle_summary.primary_workload_kind = %q", payload.Data.BundleSummary.PrimaryWorkloadKind)
	}
	if payload.Data.BundleSummary.ResourceCounts.Total != 3 {
		t.Fatalf("bundle_summary.resource_counts.total = %d", payload.Data.BundleSummary.ResourceCounts.Total)
	}
	if payload.Data.BundleSummary.Artifact.Ref == "" {
		t.Fatal("expected bundle_summary.artifact.ref")
	}
}

func TestListReleasesRequiresEnvironmentID(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseHandler{svc: stubReleaseService{}}
	r := gin.New()
	r.GET("/api/v1/releases", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases?application_id="+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListReleasesUsesApplicationAndEnvironmentFilters(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	appID := uuid.New()
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			listFn: func(_ context.Context, filter service.ReleaseListFilter) ([]*model.Release, error) {
				if filter.ApplicationID == nil || *filter.ApplicationID != appID {
					t.Fatalf("ApplicationID = %+v want %s", filter.ApplicationID, appID)
				}
				if filter.EnvironmentID != "prod" {
					t.Fatalf("EnvironmentID = %q want prod", filter.EnvironmentID)
				}
				return []*model.Release{}, nil
			},
		},
	}
	r := gin.New()
	r.GET("/api/v1/releases", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases?application_id="+appID.String()+"&environment_id=prod", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want %d", rec.Code, http.StatusOK)
	}
}

func TestGetReleaseBundlePreviewReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			getBundleFn: func(_ context.Context, id uuid.UUID) (*model.ReleaseBundlePreview, error) {
				if id != releaseID {
					t.Fatalf("id = %s want %s", id, releaseID)
				}
				return &model.ReleaseBundlePreview{
					ReleaseID:     releaseID,
					ApplicationID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
					EnvironmentID: "production",
					Namespace:     "checkout",
					RenderedBundle: model.ReleaseRenderedBundleView{
						Files: []model.ReleaseBundleFileView{
							{Path: "bundle.yaml", Content: "kind: Deployment\n"},
						},
					},
				}, nil
			},
		},
	}

	r := gin.New()
	r.GET("/api/v1/releases/:id/bundle-preview", handler.GetBundlePreview)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases/"+releaseID.String()+"/bundle-preview", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload struct {
		Data model.ReleaseBundlePreview `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Data.ReleaseID != releaseID || payload.Data.Namespace != "checkout" || len(payload.Data.RenderedBundle.Files) != 1 {
		t.Fatalf("unexpected payload: %#v", payload.Data)
	}
}

func TestGetReleaseBundlePreviewReturns404(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			getBundleFn: func(_ context.Context, _ uuid.UUID) (*model.ReleaseBundlePreview, error) {
				return nil, sql.ErrNoRows
			},
		},
	}

	r := gin.New()
	r.GET("/api/v1/releases/:id/bundle-preview", handler.GetBundlePreview)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases/"+uuid.New().String()+"/bundle-preview", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetReleaseBundlePreviewReturns409WhenBundleNotReady(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			getBundleFn: func(_ context.Context, _ uuid.UUID) (*model.ReleaseBundlePreview, error) {
				return nil, service.ErrReleaseBundleNotReady
			},
		},
	}

	r := gin.New()
	r.GET("/api/v1/releases/:id/bundle-preview", handler.GetBundlePreview)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases/"+uuid.New().String()+"/bundle-preview", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.Error.Code != "failed_precondition" {
		t.Fatalf("error code = %q, want failed_precondition", payload.Error.Code)
	}
}

func TestDeleteReleaseReturns204(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			deleteFn: func(_ context.Context, id uuid.UUID) error {
				if id != releaseID {
					t.Fatalf("id = %s want %s", id, releaseID)
				}
				return nil
			},
		},
	}
	r := gin.New()
	r.DELETE("/api/v1/releases/:id", handler.Delete)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/releases/"+releaseID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

func TestDeleteReleaseNotFoundReturns404(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			deleteFn: func(_ context.Context, _ uuid.UUID) error {
				return sql.ErrNoRows
			},
		},
	}
	r := gin.New()
	r.DELETE("/api/v1/releases/:id", handler.Delete)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/releases/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDeleteReleaseInvalidIDReturns400(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseHandler{
		svc: stubReleaseService{
			deleteFn: func(_ context.Context, _ uuid.UUID) error {
				t.Fatal("deleteFn should not be called for invalid id")
				return nil
			},
		},
	}
	r := gin.New()
	r.DELETE("/api/v1/releases/:id", handler.Delete)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/releases/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want %d", rec.Code, http.StatusBadRequest)
	}
}
