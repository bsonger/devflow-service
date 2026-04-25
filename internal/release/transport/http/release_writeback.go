package http

import (
	"context"
	"database/sql"
	"errors"
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
	if err := h.svc.UpdateStep(c.Request.Context(), releaseID, req.StepName, normalizeStepStatus(model.StepStatus(req.Status)), req.Progress, req.Message, nil, nil); err != nil {
		writeReleaseVerifyError(c, err)
		return
	}
	httpx.WriteNoContent(c)
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

func RegisterReleaseWritebackRoutes(rg *gin.RouterGroup) {
	writeback := rg.Group("/verify")
	writeback.Use(RequireObserverToken(ObserverSharedToken))
	writeback.POST("/argo/events", NewReleaseWritebackHandler().HandleArgoEvent)
	writeback.POST("/release/steps", NewReleaseWritebackHandler().HandleReleaseStep)
}

func writeReleaseVerifyError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		httpx.WriteNotFound(c, "release not found")
		return
	}
	httpx.WriteInternalError(c, err)
}
