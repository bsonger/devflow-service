package http

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubImageWritebackService struct {
	assignFn  func(context.Context, uuid.UUID, string) error
	statusFn  func(context.Context, uuid.UUID, model.ImageStatus) error
	stepFn    func(context.Context, string, string, model.StepStatus, string, *time.Time, *time.Time) error
	taskRunFn func(context.Context, string, string, string) error
	getFn     func(context.Context, uuid.UUID) (*imagedomain.Image, error)
}

func (s stubImageWritebackService) AssignPipelineID(ctx context.Context, imageID uuid.UUID, pipelineID string) error {
	if s.assignFn == nil {
		return nil
	}
	return s.assignFn(ctx, imageID, pipelineID)
}

func (s stubImageWritebackService) UpdateImageStatusByID(ctx context.Context, imageID uuid.UUID, status model.ImageStatus) error {
	if s.statusFn == nil {
		return nil
	}
	return s.statusFn(ctx, imageID, status)
}

func (s stubImageWritebackService) UpdateStepStatus(ctx context.Context, pipelineID, taskName string, status model.StepStatus, message string, start, end *time.Time) error {
	if s.stepFn == nil {
		return nil
	}
	return s.stepFn(ctx, pipelineID, taskName, status, message, start, end)
}

func (s stubImageWritebackService) BindTaskRun(ctx context.Context, pipelineID, taskName, taskRun string) error {
	if s.taskRunFn == nil {
		return nil
	}
	return s.taskRunFn(ctx, pipelineID, taskName, taskRun)
}

func (s stubImageWritebackService) Get(ctx context.Context, id uuid.UUID) (*imagedomain.Image, error) {
	if s.getFn == nil {
		return nil, sql.ErrNoRows
	}
	return s.getFn(ctx, id)
}

func TestHandleImageTaskEventRequiresToken(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ImageWritebackHandler{svc: stubImageWritebackService{}}
	r := gin.New()
	r.POST("/api/v1/images/tekton/tasks", RequireObserverToken("top-secret"), handler.HandleTektonTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/tekton/tasks", bytes.NewBufferString(`{"image_id":"11111111-1111-1111-1111-111111111111","pipeline_id":"pipe-1","task_name":"git-clone","status":"running"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleImageTaskEventUpdatesStep(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	imageID := uuid.New()
	called := false
	handler := &ImageWritebackHandler{svc: stubImageWritebackService{
		stepFn: func(_ context.Context, pipelineID, taskName string, status model.StepStatus, _ string, _, _ *time.Time) error {
			called = pipelineID == "pipe-1" && taskName == "git-clone" && status == model.StepRunning
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/images/tekton/tasks", handler.HandleTektonTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/tekton/tasks", bytes.NewBufferString(`{"image_id":"`+imageID.String()+`","pipeline_id":"pipe-1","task_name":"git-clone","status":"running"}`))
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

func TestHandleImageStatusEventUpdatesImage(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	imageID := uuid.New()
	called := false
	handler := &ImageWritebackHandler{svc: stubImageWritebackService{
		assignFn: func(_ context.Context, got uuid.UUID, pipelineID string) error {
			if got != imageID || pipelineID != "pipe-1" {
				t.Fatalf("unexpected assign args: %s %s", got, pipelineID)
			}
			return nil
		},
		statusFn: func(_ context.Context, got uuid.UUID, status model.ImageStatus) error {
			called = got == imageID && status == model.ImageRunning
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/images/tekton/status", handler.HandleTektonStatus)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/tekton/status", bytes.NewBufferString(`{"image_id":"`+imageID.String()+`","pipeline_id":"pipe-1","status":"running"}`))
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
