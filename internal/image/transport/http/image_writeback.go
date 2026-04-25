package http

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
	"time"

	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	imageservice "github.com/bsonger/devflow-service/internal/image/service"
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const ObserverTokenHeader = "X-Devflow-Observer-Token"
const VerifyTokenHeader = "X-Devflow-Verify-Token"

var ObserverSharedToken string

type imageWritebackService interface {
	AssignPipelineID(ctx context.Context, imageID uuid.UUID, pipelineID string) error
	UpdateImageStatusByID(ctx context.Context, imageID uuid.UUID, status model.ImageStatus) error
	UpdateStepStatus(ctx context.Context, pipelineID, taskName string, status model.StepStatus, message string, start, end *time.Time) error
	BindTaskRun(ctx context.Context, pipelineID, taskName, taskRun string) error
	Get(ctx context.Context, id uuid.UUID) (*imagedomain.Image, error)
}

type ImageWritebackHandler struct {
	svc imageWritebackService
}

type ImageTektonStatusRequest struct {
	ImageID    string            `json:"image_id" binding:"required"`
	PipelineID string            `json:"pipeline_id,omitempty"`
	Status     model.ImageStatus `json:"status" binding:"required"`
	Message    string            `json:"message,omitempty"`
}

type ImageTektonTaskRequest struct {
	ImageID    string           `json:"image_id" binding:"required"`
	PipelineID string           `json:"pipeline_id,omitempty"`
	TaskName   string           `json:"task_name" binding:"required"`
	TaskRun    string           `json:"task_run,omitempty"`
	Status     model.StepStatus `json:"status" binding:"required"`
	Message    string           `json:"message,omitempty"`
	StartTime  *time.Time       `json:"start_time,omitempty"`
	EndTime    *time.Time       `json:"end_time,omitempty"`
}

func NewImageWritebackHandler() *ImageWritebackHandler {
	return &ImageWritebackHandler{svc: imageservice.ImageService}
}

func RequireObserverToken(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		expected = strings.TrimSpace(expected)
		if expected == "" {
			c.Next()
			return
		}
		token := strings.TrimSpace(c.GetHeader(ObserverTokenHeader))
		if token == "" {
			token = strings.TrimSpace(c.GetHeader(VerifyTokenHeader))
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			httpx.WriteUnauthorized(c)
			c.Abort()
			return
		}
		c.Next()
	}
}

func resolvePipelineID(ctx *gin.Context, svc imageWritebackService, imageID uuid.UUID, pipelineID string) (string, error) {
	if strings.TrimSpace(pipelineID) != "" {
		return strings.TrimSpace(pipelineID), nil
	}
	image, err := svc.Get(ctx.Request.Context(), imageID)
	if err != nil {
		return "", err
	}
	if image.PipelineID == "" {
		return "", sql.ErrNoRows
	}
	return image.PipelineID, nil
}

// HandleTektonStatus
// @Summary Handle Tekton pipeline status callback
// @Tags Image
// @Accept json
// @Param data body ImageTektonStatusRequest true "Tekton status payload"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/images/tekton/status [post]
func (h *ImageWritebackHandler) HandleTektonStatus(c *gin.Context) {
	var req ImageTektonStatusRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	imageID, ok := httpx.ParseUUIDString(c, req.ImageID, "image_id")
	if !ok {
		return
	}
	if req.PipelineID != "" {
		if err := h.svc.AssignPipelineID(c.Request.Context(), imageID, req.PipelineID); err != nil {
			writeImageVerifyError(c, err)
			return
		}
	}
	if err := h.svc.UpdateImageStatusByID(c.Request.Context(), imageID, normalizeImageStatus(req.Status)); err != nil {
		writeImageVerifyError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

// HandleTektonTask
// @Summary Handle Tekton task callback
// @Tags Image
// @Accept json
// @Param data body ImageTektonTaskRequest true "Tekton task payload"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/images/tekton/tasks [post]
func (h *ImageWritebackHandler) HandleTektonTask(c *gin.Context) {
	var req ImageTektonTaskRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	imageID, ok := httpx.ParseUUIDString(c, req.ImageID, "image_id")
	if !ok {
		return
	}
	pipelineID, err := resolvePipelineID(c, h.svc, imageID, req.PipelineID)
	if err != nil {
		writeImageVerifyError(c, err)
		return
	}
	if req.TaskRun != "" {
		if err := h.svc.BindTaskRun(c.Request.Context(), pipelineID, req.TaskName, req.TaskRun); err != nil {
			writeImageVerifyError(c, err)
			return
		}
	}
	if err := h.svc.UpdateStepStatus(c.Request.Context(), pipelineID, req.TaskName, normalizeStepStatus(req.Status), req.Message, req.StartTime, req.EndTime); err != nil {
		writeImageVerifyError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func normalizeImageStatus(status model.ImageStatus) model.ImageStatus {
	switch strings.ToLower(string(status)) {
	case "pending":
		return model.ImagePending
	case "running":
		return model.ImageRunning
	case "succeeded":
		return model.ImageSucceeded
	case "failed":
		return model.ImageFailed
	default:
		return status
	}
}

func normalizeStepStatus(status model.StepStatus) model.StepStatus {
	switch strings.ToLower(string(status)) {
	case "pending":
		return model.StepPending
	case "running":
		return model.StepRunning
	case "succeeded":
		return model.StepSucceeded
	case "failed":
		return model.StepFailed
	default:
		return status
	}
}

func writeImageVerifyError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		httpx.WriteNotFound(c, "image not found")
		return
	}
	httpx.WriteInternalError(c, err)
}
