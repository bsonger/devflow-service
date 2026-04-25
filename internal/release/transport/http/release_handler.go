package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/platform/httpx"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/release/service"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/bsonger/devflow-service/internal/release/transport/runtime"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type releaseService interface {
	Create(ctx context.Context, release *model.Release) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Release, error)
	List(ctx context.Context, filter service.ReleaseListFilter) ([]*model.Release, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ReleaseHandler struct {
	svc releaseService
}

type ReleaseResponse struct {
	Data *ReleaseDoc `json:"data"`
}

type ReleaseListResponse struct {
	Data       []*ReleaseDoc    `json:"data"`
	Pagination httpx.Pagination `json:"pagination"`
}

func NewReleaseHandler(svc releaseService) *ReleaseHandler {
	return &ReleaseHandler{svc: svc}
}

func (h *ReleaseHandler) RegisterRoutes(rg *gin.RouterGroup) {
	release := rg.Group("/releases")
	release.GET("", h.List)
	release.GET("/:id", h.Get)
	release.POST("", h.Create)
	release.DELETE("/:id", h.Delete)
}

type CreateReleaseRequest struct {
	ManifestID uuid.UUID `json:"manifest_id"`
	Env        string    `json:"env,omitempty"`
	Type       string    `json:"type,omitempty"`
}

// Create
// @Summary 创建Release
// @Description 创建一个新的Release
// @Tags Release
// @Accept json
// @Produce json
// @Param data body CreateReleaseRequest true "Release Data"
// @Success 201 {object} ReleaseResponse
// @Router /api/v1/releases [post]
func (h *ReleaseHandler) Create(c *gin.Context) {
	var req CreateReleaseRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	release := &model.Release{
		ManifestID: req.ManifestID,
		Env:        req.Env,
		Type:       req.Type,
	}
	release.WithCreateDefault()
	_, err := h.svc.Create(c.Request.Context(), release)
	if err != nil {
		if errors.Is(err, service.ErrImageMissingRuntimeSpecRevision) || errors.Is(err, service.ErrRuntimeSpecBindingMismatch) || errors.Is(err, service.ErrReleaseManifestNotReady) || errors.Is(err, runtimeclient.ErrRuntimeServiceUnavailable) || errors.Is(err, releasesupport.ErrDeployTargetClusterNotReady) || errors.Is(err, releasesupport.ErrDeployTargetClusterReadinessMalformed) {
			httpx.WriteFailedPrecondition(c, http.StatusConflict, err.Error())
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusCreated, release)
}

// Get
// @Summary 获取Release
// @Tags Release
// @Param id path string true "Release ID"
// @Success 200 {object} ReleaseResponse
// @Router /api/v1/releases/{id} [get]
func (h *ReleaseHandler) Get(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	release, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusOK, release)
}

// Delete
// @Summary Delete release
// @Tags Release
// @Param id path string true "Release ID"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/releases/{id} [delete]
func (h *ReleaseHandler) Delete(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

// List
// @Summary 获取Release列表
// @Tags Release
// @Success 200 {object} ReleaseListResponse
// @Router /api/v1/releases [get]
func (h *ReleaseHandler) List(c *gin.Context) {
	filter := service.ReleaseListFilter{IncludeDeleted: httpx.IncludeDeleted(c)}
	applicationID, ok := httpx.ParseUUIDQuery(c, "application_id")
	if !ok {
		return
	}
	if applicationID != nil {
		filter.ApplicationID = applicationID
	}
	manifestID, ok := httpx.ParseUUIDQuery(c, "manifest_id")
	if !ok {
		return
	}
	if manifestID != nil {
		filter.ManifestID = manifestID
	}
	imageID, ok := httpx.ParseUUIDQuery(c, "image_id")
	if !ok {
		return
	}
	if imageID != nil {
		filter.ImageID = imageID
	}
	if status := c.Query("status"); status != "" {
		filter.Status = status
	}
	if releaseType := c.Query("type"); releaseType != "" {
		filter.Type = releaseType
	}
	releases, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, releases)
}
