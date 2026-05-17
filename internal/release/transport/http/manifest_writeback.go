package http

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	manifestservice "github.com/bsonger/devflow-service/internal/manifest/service"
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type manifestWritebackService interface {
	AssignPipelineID(ctx context.Context, manifestID uuid.UUID, pipelineID string) error
	UpdateManifestStatusByID(ctx context.Context, manifestID uuid.UUID, status model.ManifestStatus) error
	UpdateStepStatus(ctx context.Context, pipelineID, taskName string, status model.StepStatus, message string, start, end *time.Time) error
	BindTaskRun(ctx context.Context, pipelineID, taskName, taskRun string) error
	UpdateBuildResult(ctx context.Context, pipelineID, commitHash, imageRef, imageTag, imageDigest string) error
	Get(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error)
}

type ManifestWritebackHandler struct {
	svc manifestWritebackService
}

type ManifestTektonStatusRequest struct {
	ManifestID string               `json:"manifest_id" binding:"required"`
	PipelineID string               `json:"pipeline_id,omitempty"`
	Status     model.ManifestStatus `json:"status" binding:"required"`
	Message    string               `json:"message,omitempty"`
}

type ManifestTektonTaskRequest struct {
	ManifestID string           `json:"manifest_id" binding:"required"`
	PipelineID string           `json:"pipeline_id,omitempty"`
	TaskName   string           `json:"task_name" binding:"required"`
	TaskRun    string           `json:"task_run,omitempty"`
	Status     model.StepStatus `json:"status" binding:"required"`
	Message    string           `json:"message,omitempty"`
	StartTime  *time.Time       `json:"start_time,omitempty"`
	EndTime    *time.Time       `json:"end_time,omitempty"`
}

type ManifestTektonResultRequest struct {
	ManifestID  string `json:"manifest_id" binding:"required"`
	PipelineID  string `json:"pipeline_id,omitempty"`
	CommitHash  string `json:"commit_hash,omitempty"`
	ImageRef    string `json:"image_ref,omitempty"`
	ImageTag    string `json:"image_tag,omitempty"`
	ImageDigest string `json:"image_digest,omitempty"`
}

func NewManifestWritebackHandler() *ManifestWritebackHandler {
	return &ManifestWritebackHandler{svc: manifestservice.ManifestService}
}

func RegisterManifestWritebackRoutes(rg *gin.RouterGroup) {
	handler := NewManifestWritebackHandler()
	group := rg.Group("/release/manifests/tekton")
	group.Use(RequireObserverToken(ObserverSharedToken))
	group.POST("/status", handler.HandleTektonStatus)
	group.POST("/tasks", handler.HandleTektonTask)
	group.POST("/result", handler.HandleTektonResult)
}

func (h *ManifestWritebackHandler) HandleTektonStatus(c *gin.Context) {
	var req ManifestTektonStatusRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	manifestID, ok := httpx.ParseUUIDString(c, req.ManifestID, "manifest_id")
	if !ok {
		return
	}
	if req.PipelineID != "" {
		if err := h.svc.AssignPipelineID(c.Request.Context(), manifestID, req.PipelineID); err != nil {
			writeManifestVerifyError(c, err)
			return
		}
	}
	if err := h.svc.UpdateManifestStatusByID(c.Request.Context(), manifestID, normalizeManifestStatus(req.Status)); err != nil {
		writeManifestVerifyError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func (h *ManifestWritebackHandler) HandleTektonTask(c *gin.Context) {
	var req ManifestTektonTaskRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	manifestID, ok := httpx.ParseUUIDString(c, req.ManifestID, "manifest_id")
	if !ok {
		return
	}
	pipelineID, err := resolveManifestPipelineID(c, h.svc, manifestID, req.PipelineID)
	if err != nil {
		writeManifestVerifyError(c, err)
		return
	}
	if req.TaskRun != "" {
		if err := h.svc.BindTaskRun(c.Request.Context(), pipelineID, req.TaskName, req.TaskRun); err != nil {
			writeManifestVerifyError(c, err)
			return
		}
	}
	if err := h.svc.UpdateStepStatus(c.Request.Context(), pipelineID, req.TaskName, normalizeStepStatus(req.Status), strings.TrimSpace(req.Message), req.StartTime, req.EndTime); err != nil {
		writeManifestVerifyError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func (h *ManifestWritebackHandler) HandleTektonResult(c *gin.Context) {
	var req ManifestTektonResultRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	manifestID, ok := httpx.ParseUUIDString(c, req.ManifestID, "manifest_id")
	if !ok {
		return
	}
	pipelineID, err := resolveManifestPipelineID(c, h.svc, manifestID, req.PipelineID)
	if err != nil {
		writeManifestVerifyError(c, err)
		return
	}
	if err := h.svc.UpdateBuildResult(c.Request.Context(), pipelineID, req.CommitHash, req.ImageRef, req.ImageTag, req.ImageDigest); err != nil {
		writeManifestVerifyError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func resolveManifestPipelineID(ctx *gin.Context, svc manifestWritebackService, manifestID uuid.UUID, pipelineID string) (string, error) {
	if strings.TrimSpace(pipelineID) != "" {
		return strings.TrimSpace(pipelineID), nil
	}
	manifest, err := svc.Get(ctx.Request.Context(), manifestID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(manifest.PipelineID) == "" {
		return "", sql.ErrNoRows
	}
	return strings.TrimSpace(manifest.PipelineID), nil
}

func normalizeManifestStatus(status model.ManifestStatus) model.ManifestStatus {
	switch strings.ToLower(string(status)) {
	case "pending":
		return model.ManifestPending
	case "running":
		return model.ManifestRunning
	case "available":
		return model.ManifestAvailable
	case "unavailable":
		return model.ManifestUnavailable
	case "ready":
		return model.ManifestAvailable
	case "succeeded":
		return model.ManifestAvailable
	case "failed":
		return model.ManifestUnavailable
	default:
		return status
	}
}

func writeManifestVerifyError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		httpx.WriteNotFound(c, "manifest not found")
		return
	}
	httpx.WriteInternalError(c, err)
}
