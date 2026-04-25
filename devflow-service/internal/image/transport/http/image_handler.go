package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/platform/httpx"
	imageservice "github.com/bsonger/devflow-service/internal/image/service"
	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type imageService interface {
	CreateImage(ctx context.Context, m *imagedomain.Image) (uuid.UUID, error)
	List(ctx context.Context, filter imageservice.ImageListFilter) ([]imagedomain.Image, error)
	Get(ctx context.Context, id uuid.UUID) (*imagedomain.Image, error)
	Patch(ctx context.Context, id uuid.UUID, patch *imagedomain.PatchImageRequest) error
}

type ImageHandler struct {
	svc imageService
}

func NewImageHandler(svc imageService) *ImageHandler {
	return &ImageHandler{svc: svc}
}

func (h *ImageHandler) RegisterRoutes(rg *gin.RouterGroup) {
	images := rg.Group("/images")
	images.GET("", h.List)
	images.POST("", h.Create)
	images.GET("/:id", h.Get)
	images.PATCH("/:id", h.Patch)
}

func (h *ImageHandler) Create(c *gin.Context) {
	var req imagedomain.CreateImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	image := imagedomain.Image{
		ApplicationID:           req.ApplicationID,
		ConfigurationRevisionID: req.ConfigurationRevisionID,
		RuntimeSpecRevisionID:   req.RuntimeSpecRevisionID,
		Branch:                  req.Branch,
	}
	if _, err := h.svc.CreateImage(c.Request.Context(), &image); err != nil {
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	httpx.WriteData(c, http.StatusCreated, image)
}

func (h *ImageHandler) List(c *gin.Context) {
	filter := imageservice.ImageListFilter{IncludeDeleted: httpx.IncludeDeleted(c)}
	if appID := c.Query("application_id"); appID != "" {
		id, err := uuid.Parse(appID)
		if err != nil {
			httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid application_id", nil)
			return
		}
		filter.ApplicationID = &id
	}
	if pipelineID := c.Query("pipeline_id"); pipelineID != "" {
		filter.PipelineID = pipelineID
	}
	if status := c.Query("status"); status != "" {
		filter.Status = status
	}
	if branch := c.Query("branch"); branch != "" {
		filter.Branch = branch
	}
	if name := c.Query("name"); name != "" {
		filter.Name = name
	}

	items, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	paging, err := httpx.ParsePagination(c)
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	total := len(items)
	items = httpx.PaginateSlice(items, paging)
	httpx.WriteList(c, http.StatusOK, items, paging, total)
}

func (h *ImageHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}
	image, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	httpx.WriteData(c, http.StatusOK, image)
}

func (h *ImageHandler) Patch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}
	var patch imagedomain.PatchImageRequest
	if err := c.ShouldBindJSON(&patch); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	if err := h.svc.Patch(c.Request.Context(), id, &patch); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "image not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	httpx.WriteNoContent(c)
}
