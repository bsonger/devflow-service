package http

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubReleaseWritebackService struct {
	updateStatusFn   func(context.Context, uuid.UUID, model.ReleaseStatus) error
	updateStepFn     func(context.Context, uuid.UUID, string, model.StepStatus, int32, string, *time.Time, *time.Time) error
	updateArtifactFn func(context.Context, uuid.UUID, string, string, string, string, string, model.StepStatus, int32) error
	getFn            func(context.Context, uuid.UUID) (*model.Release, error)
}

func (s stubReleaseWritebackService) UpdateStatus(ctx context.Context, releaseID uuid.UUID, status model.ReleaseStatus) error {
	if s.updateStatusFn == nil {
		return nil
	}
	return s.updateStatusFn(ctx, releaseID, status)
}

func (s stubReleaseWritebackService) UpdateStep(ctx context.Context, releaseID uuid.UUID, stepName string, status model.StepStatus, progress int32, message string, start, end *time.Time) error {
	if s.updateStepFn == nil {
		return nil
	}
	return s.updateStepFn(ctx, releaseID, stepName, status, progress, message, start, end)
}

func (s stubReleaseWritebackService) UpdateArtifact(ctx context.Context, releaseID uuid.UUID, repository, tag, digest, ref, message string, status model.StepStatus, progress int32) error {
	if s.updateArtifactFn == nil {
		return nil
	}
	return s.updateArtifactFn(ctx, releaseID, repository, tag, digest, ref, message, status, progress)
}

func (s stubReleaseWritebackService) Get(ctx context.Context, id uuid.UUID) (*model.Release, error) {
	if s.getFn == nil {
		return nil, sql.ErrNoRows
	}
	return s.getFn(ctx, id)
}

func TestRequireObserverTokenAcceptsVerifyTokenHeader(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/test", RequireObserverToken("secret"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Devflow-Verify-Token", "secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRequireObserverTokenRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/test", RequireObserverToken("secret"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleArgoEventUpdatesReleaseStatus(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	called := false
	stepCalled := false
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateStatusFn: func(_ context.Context, got uuid.UUID, status model.ReleaseStatus) error {
			called = got == releaseID && status == model.ReleaseSucceeded
			return nil
		},
		updateStepFn: func(_ context.Context, got uuid.UUID, stepName string, status model.StepStatus, progress int32, message string, _, _ *time.Time) error {
			stepCalled = got == releaseID && stepName == "observe_rollout" && status == model.StepSucceeded && progress == 100 && message == "rollout observed as succeeded by argocd"
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/argo/events", handler.HandleArgoEvent)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","status":"Succeeded"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/argo/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if !called {
		t.Fatalf("status update was not called with expected args")
	}
	if !stepCalled {
		t.Fatalf("step update was not called with expected args")
	}
}

func TestHandleArgoEventDoesNotClaimReleaseOwnedStartDeploymentStep(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	var gotStepName string
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateStatusFn: func(_ context.Context, got uuid.UUID, status model.ReleaseStatus) error {
			if got != releaseID || status != model.ReleaseRunning {
				t.Fatalf("unexpected status update: release=%s status=%s", got, status)
			}
			return nil
		},
		updateStepFn: func(_ context.Context, got uuid.UUID, stepName string, status model.StepStatus, progress int32, message string, _, _ *time.Time) error {
			if got != releaseID {
				t.Fatalf("unexpected release id: %s", got)
			}
			gotStepName = stepName
			if status != model.StepRunning || progress != 100 || message != "rollout is running in argocd" {
				t.Fatalf("unexpected step update: step=%s status=%s progress=%d message=%q", stepName, status, progress, message)
			}
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/argo/events", handler.HandleArgoEvent)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","status":"Running"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/argo/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if gotStepName != "observe_rollout" {
		t.Fatalf("stepName = %q want observe_rollout", gotStepName)
	}
	if gotStepName == "start_deployment" {
		t.Fatalf("argo callback must not own start_deployment")
	}
}

func TestHandleArgoEventMapsFailedToReleaseFailed(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	var gotStatus model.ReleaseStatus
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateStatusFn: func(_ context.Context, _ uuid.UUID, status model.ReleaseStatus) error {
			gotStatus = status
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/argo/events", handler.HandleArgoEvent)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","status":"Failed"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/argo/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNoContent)
	}
	if gotStatus != model.ReleaseFailed {
		t.Fatalf("status = %q, want Failed", gotStatus)
	}
}

func TestHandleArgoEventMapsErrorToSyncFailed(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	var gotStatus model.ReleaseStatus
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateStatusFn: func(_ context.Context, _ uuid.UUID, status model.ReleaseStatus) error {
			gotStatus = status
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/argo/events", handler.HandleArgoEvent)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","status":"Error"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/argo/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNoContent)
	}
	if gotStatus != model.ReleaseSyncFailed {
		t.Fatalf("status = %q, want SyncFailed", gotStatus)
	}
}

func TestHandleArgoEventMapsRunningToReleaseRunning(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	var gotStatus model.ReleaseStatus
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateStatusFn: func(_ context.Context, _ uuid.UUID, status model.ReleaseStatus) error {
			gotStatus = status
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/argo/events", handler.HandleArgoEvent)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","status":"Running"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/argo/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNoContent)
	}
	if gotStatus != model.ReleaseRunning {
		t.Fatalf("status = %q, want Running", gotStatus)
	}
}

func TestHandleArgoEventReturns400ForBadPayload(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{}}
	r := gin.New()
	r.POST("/api/v1/verify/argo/events", handler.HandleArgoEvent)

	body := bytes.NewBufferString(`{"release_id":"bad-uuid","status":"Succeeded"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/argo/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleArgoEventReturns404ForMissingRelease(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateStatusFn: func(_ context.Context, _ uuid.UUID, _ model.ReleaseStatus) error {
			return sql.ErrNoRows
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/argo/events", handler.HandleArgoEvent)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","status":"Succeeded"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/argo/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleReleaseStepUpdatesStep(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	called := false
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateStepFn: func(_ context.Context, got uuid.UUID, stepName string, status model.StepStatus, progress int32, message string, _, _ *time.Time) error {
			called = got == releaseID && stepName == "canary_10" && status == model.StepRunning && progress == 50 && message == "in progress"
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/release/steps", handler.HandleReleaseStep)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","step_code":"canary_10","status":"Running","progress":50,"message":"in progress"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/release/steps", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if !called {
		t.Fatalf("step update was not called with expected args")
	}
}

func TestHandleReleaseArtifactUpdatesArtifactAndStep(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	called := false
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateArtifactFn: func(_ context.Context, got uuid.UUID, repository, tag, digest, ref, message string, status model.StepStatus, progress int32) error {
			called = got == releaseID &&
				repository == "registry.example.com/devflow/releases/demo-api" &&
				tag == "release-20260428" &&
				digest == "sha256:abc" &&
				ref == "oci://registry.example.com/devflow/releases/demo-api:release-20260428" &&
				message == "bundle published" &&
				status == model.StepSucceeded &&
				progress == 100
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/release/artifact", handler.HandleReleaseArtifact)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","artifact_repository":"registry.example.com/devflow/releases/demo-api","artifact_tag":"release-20260428","artifact_digest":"sha256:abc","artifact_ref":"oci://registry.example.com/devflow/releases/demo-api:release-20260428","status":"Succeeded","progress":100,"message":"bundle published"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/release/artifact", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if !called {
		t.Fatalf("artifact update was not called with expected args")
	}
}

func TestHandleReleaseStepGeneratesDefaultMessageWhenMissing(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	var gotMessage string
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateStepFn: func(_ context.Context, got uuid.UUID, stepName string, status model.StepStatus, progress int32, message string, _, _ *time.Time) error {
			if got == releaseID && stepName == "observe_rollout" && status == model.StepRunning && progress == 45 {
				gotMessage = message
			}
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/release/steps", handler.HandleReleaseStep)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","step_code":"observe_rollout","status":"Running","progress":45}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/release/steps", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if gotMessage != "observe rollout running (45%)" {
		t.Fatalf("message = %q", gotMessage)
	}
}

func TestHandleReleaseStepNormalizesLegacyStepNameMessageWhenMissing(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	var gotStepName string
	var gotMessage string
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateStepFn: func(_ context.Context, got uuid.UUID, stepName string, status model.StepStatus, progress int32, message string, _, _ *time.Time) error {
			if got == releaseID && status == model.StepSucceeded {
				gotStepName = stepName
				gotMessage = message
			}
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/release/steps", handler.HandleReleaseStep)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","step_name":"publish_bundle","status":"Succeeded","progress":100}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/release/steps", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if gotStepName != "publish_bundle" {
		t.Fatalf("stepName = %q", gotStepName)
	}
	if gotMessage != "publish bundle succeeded" {
		t.Fatalf("message = %q", gotMessage)
	}
}

func TestHandleReleaseArtifactGeneratesDefaultMessageWhenMissing(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	var gotMessage string
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateArtifactFn: func(_ context.Context, got uuid.UUID, repository, tag, digest, ref, message string, status model.StepStatus, progress int32) error {
			if got == releaseID && status == model.StepSucceeded {
				gotMessage = message
			}
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/release/artifact", handler.HandleReleaseArtifact)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","artifact_ref":"oci://registry.example.com/devflow/releases/demo-api@sha256:abc","status":"Succeeded","progress":100}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/release/artifact", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if gotMessage != "deployment bundle artifact recorded: oci://registry.example.com/devflow/releases/demo-api@sha256:abc" {
		t.Fatalf("message = %q", gotMessage)
	}
}

func TestHandleReleaseArtifactGeneratesRunningMessageWhenMissing(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	var gotMessage string
	var gotStatus model.StepStatus
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateArtifactFn: func(_ context.Context, got uuid.UUID, repository, tag, digest, ref, message string, status model.StepStatus, progress int32) error {
			if got == releaseID {
				gotMessage = message
				gotStatus = status
			}
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/release/artifact", handler.HandleReleaseArtifact)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","artifact_ref":"oci://registry.example.com/devflow/releases/demo-api:release-20260428","status":"running","progress":30}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/release/artifact", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if gotStatus != model.StepRunning {
		t.Fatalf("status = %q", gotStatus)
	}
	if gotMessage != "recording deployment bundle artifact: oci://registry.example.com/devflow/releases/demo-api:release-20260428" {
		t.Fatalf("message = %q", gotMessage)
	}
}

func TestHandleReleaseArtifactReturns404ForMissingRelease(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateArtifactFn: func(_ context.Context, _ uuid.UUID, _, _, _, _, _ string, _ model.StepStatus, _ int32) error {
			return sql.ErrNoRows
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/release/artifact", handler.HandleReleaseArtifact)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","artifact_ref":"oci://registry.example.com/demo:tag"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/release/artifact", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleReleaseStepReturns400ForMissingField(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{}}
	r := gin.New()
	r.POST("/api/v1/verify/release/steps", handler.HandleReleaseStep)

	body := bytes.NewBufferString(`{"release_id":"` + uuid.New().String() + `","status":"Succeeded"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/release/steps", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleReleaseStepAcceptsLegacyStepName(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	called := false
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateStepFn: func(_ context.Context, got uuid.UUID, stepName string, status model.StepStatus, progress int32, message string, _, _ *time.Time) error {
			called = got == releaseID && stepName == "deploy" && status == model.StepSucceeded && progress == 100 && message == "done"
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/release/steps", handler.HandleReleaseStep)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","step_name":"deploy","status":"Succeeded","progress":100,"message":"done"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/release/steps", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if !called {
		t.Fatalf("legacy step_name update was not called with expected args")
	}
}

func TestHandleReleaseStepReturns404ForMissingRelease(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateStepFn: func(_ context.Context, _ uuid.UUID, _ string, _ model.StepStatus, _ int32, _ string, _, _ *time.Time) error {
			return sql.ErrNoRows
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/release/steps", handler.HandleReleaseStep)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","step_name":"deploy","status":"Succeeded"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/release/steps", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleReleaseStepNormalizesStepStatus(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	releaseID := uuid.New()
	var gotStatus model.StepStatus
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{
		updateStepFn: func(_ context.Context, _ uuid.UUID, _ string, status model.StepStatus, _ int32, _ string, _, _ *time.Time) error {
			gotStatus = status
			return nil
		},
	}}
	r := gin.New()
	r.POST("/api/v1/verify/release/steps", handler.HandleReleaseStep)

	body := bytes.NewBufferString(`{"release_id":"` + releaseID.String() + `","step_name":"deploy","status":"succeeded"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/release/steps", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d want %d", rec.Code, http.StatusNoContent)
	}
	if gotStatus != model.StepSucceeded {
		t.Fatalf("status = %q, want Succeeded", gotStatus)
	}
}

func TestHandleArgoEventRequiresToken(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{}}
	r := gin.New()
	r.POST("/api/v1/verify/argo/events", RequireObserverToken("top-secret"), handler.HandleArgoEvent)

	body := bytes.NewBufferString(`{"release_id":"` + uuid.New().String() + `","status":"Succeeded"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/argo/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleReleaseStepRequiresToken(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := &ReleaseWritebackHandler{svc: stubReleaseWritebackService{}}
	r := gin.New()
	r.POST("/api/v1/verify/release/steps", RequireObserverToken("top-secret"), handler.HandleReleaseStep)

	body := bytes.NewBufferString(`{"release_id":"` + uuid.New().String() + `","step_name":"deploy","status":"Succeeded"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/release/steps", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want %d", rec.Code, http.StatusUnauthorized)
	}
}
