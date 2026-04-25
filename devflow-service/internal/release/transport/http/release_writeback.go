package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
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

func (h *ReleaseWritebackHandler) HandleArgoEvent(c *gin.Context) {
	var req ArgoEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	releaseID, err := uuid.Parse(req.ReleaseID)
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid release_id", nil)
		return
	}
	status := mapArgoStatusToReleaseStatus(req.Status)
	if err := h.svc.UpdateStatus(c.Request.Context(), releaseID, status); err != nil {
		writeReleaseVerifyError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func (h *ReleaseWritebackHandler) HandleReleaseStep(c *gin.Context) {
	var req ReleaseStepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	releaseID, err := uuid.Parse(req.ReleaseID)
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid release_id", nil)
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
		httpx.WriteError(c, http.StatusNotFound, "not_found", "release not found", nil)
		return
	}
	httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
}
