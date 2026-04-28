package http

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubManifestWritebackService struct {
	assignFn      func(context.Context, uuid.UUID, string) error
	statusFn      func(context.Context, uuid.UUID, model.ManifestStatus) error
	stepFn        func(context.Context, string, string, model.StepStatus, string, *time.Time, *time.Time) error
	taskRunFn     func(context.Context, string, string, string) error
	buildResultFn func(context.Context, string, string, string, string, string) error
	getFn         func(context.Context, uuid.UUID) (*manifestdomain.Manifest, error)
}

func (s stubManifestWritebackService) AssignPipelineID(ctx context.Context, manifestID uuid.UUID, pipelineID string) error {
	if s.assignFn == nil {
		return nil
	}
	return s.assignFn(ctx, manifestID, pipelineID)
}

func (s stubManifestWritebackService) UpdateManifestStatusByID(ctx context.Context, manifestID uuid.UUID, status model.ManifestStatus) error {
	if s.statusFn == nil {
		return nil
	}
	return s.statusFn(ctx, manifestID, status)
}

func (s stubManifestWritebackService) UpdateStepStatus(ctx context.Context, pipelineID, taskName string, status model.StepStatus, message string, start, end *time.Time) error {
	if s.stepFn == nil {
		return nil
	}
	return s.stepFn(ctx, pipelineID, taskName, status, message, start, end)
}

func (s stubManifestWritebackService) BindTaskRun(ctx context.Context, pipelineID, taskName, taskRun string) error {
	if s.taskRunFn == nil {
		return nil
	}
	return s.taskRunFn(ctx, pipelineID, taskName, taskRun)
}

func (s stubManifestWritebackService) UpdateBuildResult(ctx context.Context, pipelineID, commitHash, imageRef, imageTag, imageDigest string) error {
	if s.buildResultFn == nil {
		return nil
	}
	return s.buildResultFn(ctx, pipelineID, commitHash, imageRef, imageTag, imageDigest)
}

func (s stubManifestWritebackService) Get(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
	if s.getFn == nil {
		return nil, sql.ErrNoRows
	}
	return s.getFn(ctx, id)
}

func TestHandleManifestTaskEventRequiresToken(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ManifestWritebackHandler{svc: stubManifestWritebackService{}}
	r := gin.New()
	r.POST("/api/v1/manifests/tekton/tasks", RequireManifestObserverToken("top-secret"), handler.HandleTektonTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manifests/tekton/tasks", bytes.NewBufferString(`{"manifest_id":"11111111-1111-1111-1111-111111111111","pipeline_id":"pipe-1","task_name":"git-clone","status":"running"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleManifestTaskEventUpdatesStep(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	manifestID := uuid.New()
	called := false
	handler := &ManifestWritebackHandler{svc: stubManifestWritebackService{
		stepFn: func(_ context.Context, pipelineID, taskName string, status model.StepStatus, _ string, _, _ *time.Time) error {
			called = pipelineID == "pipe-1" && taskName == "git-clone" && status == model.StepRunning
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/manifests/tekton/tasks", handler.HandleTektonTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manifests/tekton/tasks", bytes.NewBufferString(`{"manifest_id":"`+manifestID.String()+`","pipeline_id":"pipe-1","task_name":"git-clone","status":"running"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if !called {
		t.Fatalf("step update was not called")
	}
}

func TestHandleManifestStatusEventUpdatesManifest(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	manifestID := uuid.New()
	called := false
	handler := &ManifestWritebackHandler{svc: stubManifestWritebackService{
		assignFn: func(_ context.Context, got uuid.UUID, pipelineID string) error {
			if got != manifestID || pipelineID != "pipe-1" {
				t.Fatalf("unexpected assign args: %s %s", got, pipelineID)
			}
			return nil
		},
		statusFn: func(_ context.Context, got uuid.UUID, status model.ManifestStatus) error {
			called = got == manifestID && status == model.ManifestRunning
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/manifests/tekton/status", handler.HandleTektonStatus)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manifests/tekton/status", bytes.NewBufferString(`{"manifest_id":"`+manifestID.String()+`","pipeline_id":"pipe-1","status":"running"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if !called {
		t.Fatalf("status update was not called")
	}
}

func TestHandleManifestResultEventUpdatesBuildResult(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	manifestID := uuid.New()
	called := false
	handler := &ManifestWritebackHandler{svc: stubManifestWritebackService{
		buildResultFn: func(_ context.Context, pipelineID, commitHash, imageRef, imageTag, imageDigest string) error {
			called = pipelineID == "pipe-1" &&
				commitHash == "abcdef123456" &&
				imageRef == "repo/demo@sha256:abc" &&
				imageTag == "20260427-120000" &&
				imageDigest == "sha256:abc"
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/manifests/tekton/result", handler.HandleTektonResult)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manifests/tekton/result", bytes.NewBufferString(`{"manifest_id":"`+manifestID.String()+`","pipeline_id":"pipe-1","commit_hash":"abcdef123456","image_ref":"repo/demo@sha256:abc","image_tag":"20260427-120000","image_digest":"sha256:abc"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if !called {
		t.Fatalf("build result update was not called")
	}
}
