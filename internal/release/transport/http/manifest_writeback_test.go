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
	assignPipelineIDFn      func(context.Context, uuid.UUID, string) error
	updateManifestStatusFn  func(context.Context, uuid.UUID, model.ManifestStatus) error
	updateStepStatusFn      func(context.Context, string, string, model.StepStatus, string, *time.Time, *time.Time) error
	bindTaskRunFn           func(context.Context, string, string, string) error
	updateBuildResultFn     func(context.Context, string, string, string, string, string) error
	getFn                   func(context.Context, uuid.UUID) (*manifestdomain.Manifest, error)
}

func (s stubManifestWritebackService) AssignPipelineID(ctx context.Context, manifestID uuid.UUID, pipelineID string) error {
	return s.assignPipelineIDFn(ctx, manifestID, pipelineID)
}

func (s stubManifestWritebackService) UpdateManifestStatusByID(ctx context.Context, manifestID uuid.UUID, status model.ManifestStatus) error {
	return s.updateManifestStatusFn(ctx, manifestID, status)
}

func (s stubManifestWritebackService) UpdateStepStatus(ctx context.Context, pipelineID, taskName string, status model.StepStatus, message string, start, end *time.Time) error {
	return s.updateStepStatusFn(ctx, pipelineID, taskName, status, message, start, end)
}

func (s stubManifestWritebackService) BindTaskRun(ctx context.Context, pipelineID, taskName, taskRun string) error {
	return s.bindTaskRunFn(ctx, pipelineID, taskName, taskRun)
}

func (s stubManifestWritebackService) UpdateBuildResult(ctx context.Context, pipelineID, commitHash, imageRef, imageTag, imageDigest string) error {
	return s.updateBuildResultFn(ctx, pipelineID, commitHash, imageRef, imageTag, imageDigest)
}

func (s stubManifestWritebackService) Get(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
	return s.getFn(ctx, id)
}

func TestHandleTektonStatusUpdatesManifestByNewReleasePath(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	manifestID := uuid.New()
	assignCalled := false
	statusCalled := false
	handler := &ManifestWritebackHandler{
		svc: stubManifestWritebackService{
			assignPipelineIDFn: func(_ context.Context, gotManifestID uuid.UUID, pipelineID string) error {
				assignCalled = gotManifestID == manifestID && pipelineID == "pipe-1"
				return nil
			},
			updateManifestStatusFn: func(_ context.Context, gotManifestID uuid.UUID, status model.ManifestStatus) error {
				statusCalled = gotManifestID == manifestID && status == model.ManifestAvailable
				return nil
			},
		},
	}
	r := gin.New()
	r.POST("/api/v1/release/manifests/tekton/status", RequireObserverToken("top-secret"), handler.HandleTektonStatus)
	body := bytes.NewBufferString(`{"manifest_id":"` + manifestID.String() + `","pipeline_id":"pipe-1","status":"Succeeded"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/release/manifests/tekton/status", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ObserverTokenHeader, "top-secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !assignCalled {
		t.Fatal("AssignPipelineID was not called")
	}
	if !statusCalled {
		t.Fatal("UpdateManifestStatusByID was not called")
	}
}

func TestHandleTektonTaskResolvesPipelineAndUpdatesTaskState(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	manifestID := uuid.New()
	bindCalled := false
	stepCalled := false
	handler := &ManifestWritebackHandler{
		svc: stubManifestWritebackService{
			getFn: func(_ context.Context, gotManifestID uuid.UUID) (*manifestdomain.Manifest, error) {
				if gotManifestID != manifestID {
					t.Fatalf("manifest_id = %s want %s", gotManifestID, manifestID)
				}
				return &manifestdomain.Manifest{PipelineID: "pipe-2"}, nil
			},
			bindTaskRunFn: func(_ context.Context, pipelineID, taskName, taskRun string) error {
				bindCalled = pipelineID == "pipe-2" && taskName == "git-clone" && taskRun == "taskrun-1"
				return nil
			},
			updateStepStatusFn: func(_ context.Context, pipelineID, taskName string, status model.StepStatus, message string, _, _ *time.Time) error {
				stepCalled = pipelineID == "pipe-2" && taskName == "git-clone" && status == model.StepSucceeded && message == "done"
				return nil
			},
		},
	}
	r := gin.New()
	r.POST("/api/v1/release/manifests/tekton/tasks", RequireObserverToken("top-secret"), handler.HandleTektonTask)
	body := bytes.NewBufferString(`{"manifest_id":"` + manifestID.String() + `","task_name":"git-clone","task_run":"taskrun-1","status":"Succeeded","message":"done"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/release/manifests/tekton/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ObserverTokenHeader, "top-secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bindCalled {
		t.Fatal("BindTaskRun was not called")
	}
	if !stepCalled {
		t.Fatal("UpdateStepStatus was not called")
	}
}

func TestHandleTektonResultUpdatesBuildResult(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	manifestID := uuid.New()
	resultCalled := false
	handler := &ManifestWritebackHandler{
		svc: stubManifestWritebackService{
			getFn: func(_ context.Context, _ uuid.UUID) (*manifestdomain.Manifest, error) {
				return &manifestdomain.Manifest{PipelineID: "pipe-3"}, nil
			},
			updateBuildResultFn: func(_ context.Context, pipelineID, commitHash, imageRef, imageTag, imageDigest string) error {
				resultCalled = pipelineID == "pipe-3" &&
					commitHash == "abcdef123456" &&
					imageRef == "registry.example.com/devflow/demo-api@sha256:abc" &&
					imageTag == "release-1" &&
					imageDigest == "sha256:abc"
				return nil
			},
		},
	}
	r := gin.New()
	r.POST("/api/v1/release/manifests/tekton/result", RequireObserverToken("top-secret"), handler.HandleTektonResult)
	body := bytes.NewBufferString(`{"manifest_id":"` + manifestID.String() + `","commit_hash":"abcdef123456","image_ref":"registry.example.com/devflow/demo-api@sha256:abc","image_tag":"release-1","image_digest":"sha256:abc"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/release/manifests/tekton/result", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ObserverTokenHeader, "top-secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !resultCalled {
		t.Fatal("UpdateBuildResult was not called")
	}
}

func TestHandleTektonStatusReturnsNotFoundWhenManifestMissing(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ManifestWritebackHandler{
		svc: stubManifestWritebackService{
			assignPipelineIDFn: func(_ context.Context, _ uuid.UUID, _ string) error {
				return sql.ErrNoRows
			},
			updateManifestStatusFn: func(_ context.Context, _ uuid.UUID, _ model.ManifestStatus) error {
				return nil
			},
		},
	}
	r := gin.New()
	r.POST("/api/v1/release/manifests/tekton/status", RequireObserverToken("top-secret"), handler.HandleTektonStatus)
	body := bytes.NewBufferString(`{"manifest_id":"` + uuid.New().String() + `","pipeline_id":"pipe-1","status":"Succeeded"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/release/manifests/tekton/status", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ObserverTokenHeader, "top-secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
