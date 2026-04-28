package http

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/httpx"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/release/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type releaseWritebackService interface {
	UpdateStatus(ctx context.Context, releaseID uuid.UUID, status model.ReleaseStatus) error
	UpdateStep(ctx context.Context, releaseID uuid.UUID, stepName string, status model.StepStatus, progress int32, message string, start, end *time.Time) error
	UpdateArtifact(ctx context.Context, releaseID uuid.UUID, repository, tag, digest, ref, message string, status model.StepStatus, progress int32) error
	Get(ctx context.Context, id uuid.UUID) (*model.Release, error)
}

type ReleaseWritebackHandler struct {
	svc releaseWritebackService
}

func NewReleaseWritebackHandler() *ReleaseWritebackHandler {
	return &ReleaseWritebackHandler{svc: service.ReleaseService}
}

// HandleArgoEvent
// @Summary Handle Argo CD event callback
// @Tags Release
// @Accept json
// @Param data body ArgoEventRequest true "Argo event payload"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/verify/argo/events [post]
func (h *ReleaseWritebackHandler) HandleArgoEvent(c *gin.Context) {
	var req ArgoEventRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	releaseID, ok := httpx.ParseUUIDString(c, req.ReleaseID, "release_id")
	if !ok {
		return
	}
	status := mapArgoStatusToReleaseStatus(req.Status)
	if err := h.svc.UpdateStatus(c.Request.Context(), releaseID, status); err != nil {
		writeReleaseVerifyError(c, err)
		return
	}
	if stepCode, stepStatus, stepMessage, ok := deriveArgoStepUpdate(req.Status); ok {
		if err := h.svc.UpdateStep(c.Request.Context(), releaseID, stepCode, stepStatus, 100, stepMessage, nil, nil); err != nil {
			writeReleaseVerifyError(c, err)
			return
		}
	}
	httpx.WriteNoContent(c)
}

// HandleReleaseStep
// @Summary Handle release step status callback
// @Tags Release
// @Accept json
// @Param data body ReleaseStepRequest true "Release step payload"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/verify/release/steps [post]
func (h *ReleaseWritebackHandler) HandleReleaseStep(c *gin.Context) {
	var req ReleaseStepRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	releaseID, ok := httpx.ParseUUIDString(c, req.ReleaseID, "release_id")
	if !ok {
		return
	}
	stepKey, err := resolveReleaseStepKey(req.StepCode, req.StepName)
	if err != nil {
		httpx.WriteInvalidArgument(c, err.Error())
		return
	}
	status := normalizeStepStatus(model.StepStatus(req.Status))
	message := normalizedReleaseStepMessage(stepKey, status, req.Progress, req.Message)
	if err := h.svc.UpdateStep(c.Request.Context(), releaseID, stepKey, status, req.Progress, message, nil, nil); err != nil {
		writeReleaseVerifyError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

// HandleReleaseArtifact
// @Summary Handle release artifact callback
// @Tags Release
// @Accept json
// @Param data body ReleaseArtifactRequest true "Release artifact payload"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/verify/release/artifact [post]
func (h *ReleaseWritebackHandler) HandleReleaseArtifact(c *gin.Context) {
	var req ReleaseArtifactRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	releaseID, ok := httpx.ParseUUIDString(c, req.ReleaseID, "release_id")
	if !ok {
		return
	}
	if err := h.svc.UpdateArtifact(
		c.Request.Context(),
		releaseID,
		req.ArtifactRepository,
		req.ArtifactTag,
		req.ArtifactDigest,
		req.ArtifactRef,
		normalizedArtifactMessage(req),
		normalizeArtifactStepStatus(req.Status),
		req.Progress,
	); err != nil {
		writeReleaseVerifyError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func resolveReleaseStepKey(stepCode, stepName string) (string, error) {
	if key := strings.TrimSpace(stepCode); key != "" {
		return key, nil
	}
	if name := strings.TrimSpace(stepName); name != "" {
		return name, nil
	}
	return "", fmt.Errorf("step_code or step_name is required")
}

func mapArgoStatusToReleaseStatus(phase string) model.ReleaseStatus {
	switch strings.ToLower(phase) {
	case "succeeded":
		return model.ReleaseSucceeded
	case "failed":
		return model.ReleaseFailed
	case "error":
		return model.ReleaseSyncFailed
	case "running":
		return model.ReleaseRunning
	default:
		return model.ReleaseStatus(phase)
	}
}

func deriveArgoStepUpdate(phase string) (string, model.StepStatus, string, bool) {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "succeeded":
		return "observe_rollout", model.StepSucceeded, "rollout observed as succeeded by argocd", true
	case "failed":
		return "observe_rollout", model.StepFailed, "rollout observed as failed by argocd", true
	case "error":
		return "observe_rollout", model.StepFailed, "rollout observed as error by argocd", true
	case "running":
		return "observe_rollout", model.StepRunning, "rollout is running in argocd", true
	default:
		return "", "", "", false
	}
}

func normalizedReleaseStepMessage(stepCode string, status model.StepStatus, progress int32, message string) string {
	if trimmed := strings.TrimSpace(message); trimmed != "" {
		return trimmed
	}
	label := strings.ReplaceAll(strings.TrimSpace(stepCode), "_", " ")
	switch status {
	case model.StepRunning:
		if progress > 0 {
			return fmt.Sprintf("%s running (%d%%)", label, progress)
		}
		return fmt.Sprintf("%s running", label)
	case model.StepSucceeded:
		return fmt.Sprintf("%s succeeded", label)
	case model.StepFailed:
		return fmt.Sprintf("%s failed", label)
	case model.StepPending:
		return fmt.Sprintf("%s pending", label)
	default:
		if label == "" {
			return ""
		}
		return label
	}
}

func normalizedArtifactMessage(req ReleaseArtifactRequest) string {
	if trimmed := strings.TrimSpace(req.Message); trimmed != "" {
		return trimmed
	}
	status := normalizeArtifactStepStatus(req.Status)
	ref := strings.TrimSpace(req.ArtifactRef)
	switch status {
	case model.StepSucceeded:
		if ref != "" {
			return fmt.Sprintf("deployment bundle artifact recorded: %s", ref)
		}
		return "deployment bundle artifact recorded"
	case model.StepRunning:
		if ref != "" {
			return fmt.Sprintf("recording deployment bundle artifact: %s", ref)
		}
		return "recording deployment bundle artifact"
	case model.StepFailed:
		if ref != "" {
			return fmt.Sprintf("deployment bundle artifact failed: %s", ref)
		}
		return "deployment bundle artifact failed"
	default:
		return ""
	}
}

func RegisterReleaseWritebackRoutes(rg *gin.RouterGroup) {
	writeback := rg.Group("/verify")
	writeback.Use(RequireObserverToken(ObserverSharedToken))
	writeback.POST("/argo/events", NewReleaseWritebackHandler().HandleArgoEvent)
	writeback.POST("/release/steps", NewReleaseWritebackHandler().HandleReleaseStep)
	writeback.POST("/release/artifact", NewReleaseWritebackHandler().HandleReleaseArtifact)
}

func normalizeArtifactStepStatus(status string) model.StepStatus {
	status = strings.TrimSpace(status)
	if status == "" {
		return ""
	}
	return normalizeStepStatus(model.StepStatus(status))
}

func writeReleaseVerifyError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		httpx.WriteNotFound(c, "release not found")
		return
	}
	httpx.WriteInternalError(c, err)
}
