package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	manifestservice "github.com/bsonger/devflow-service/internal/manifest/service"
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type manifestService interface {
	CreateManifest(context.Context, *manifestdomain.CreateManifestRequest) (*manifestdomain.Manifest, error)
	List(context.Context, manifestdomain.ManifestListFilter) ([]manifestdomain.Manifest, error)
	Get(context.Context, uuid.UUID) (*manifestdomain.Manifest, error)
	GetResources(context.Context, uuid.UUID) (*manifestdomain.ManifestResourcesView, error)
	Delete(context.Context, uuid.UUID) error
}

type ManifestHandler struct {
	svc manifestService
}

type ManifestResponse struct {
	Data *ManifestDoc `json:"data"`
}

type ManifestListResponse struct {
	Data       []ManifestDoc    `json:"data"`
	Pagination httpx.Pagination `json:"pagination"`
}

type ManifestResourcesResponse struct {
	Data *ManifestResourcesViewDoc `json:"data"`
}

func NewManifestHandler(svc manifestService) *ManifestHandler {
	return &ManifestHandler{svc: svc}
}

func (h *ManifestHandler) RegisterRoutes(rg *gin.RouterGroup) {
	manifests := rg.Group("/manifests")
	manifests.POST("", h.Create)
	manifests.GET("", h.List)
	manifests.GET("/:id", h.Get)
	manifests.GET("/:id/resources", h.GetResources)
	manifests.DELETE("/:id", h.Delete)
}

// CreateManifest godoc
// @Summary Create manifest
// @Tags Manifest
// @Accept json
// @Produce json
// @Param data body CreateManifestRequestDoc true "Manifest create request"
// @Success 201 {object} ManifestResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/manifests [post]
func (h *ManifestHandler) Create(c *gin.Context) {
	var req manifestdomain.CreateManifestRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	item, err := h.svc.CreateManifest(c.Request.Context(), &req)
	if err != nil {
		writeManifestError(c, err)
		return
	}
	httpx.WriteData(c, http.StatusCreated, item)
}

// ListManifests godoc
// @Summary List manifests
// @Tags Manifest
// @Produce json
// @Param application_id query string false "Application ID"
// @Param image_id query string false "Image ID"
// @Param page query int false "Page"
// @Param page_size query int false "Page size"
// @Success 200 {object} ManifestListResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/manifests [get]
func (h *ManifestHandler) List(c *gin.Context) {
	filter := manifestdomain.ManifestListFilter{IncludeDeleted: httpx.IncludeDeleted(c)}
	applicationID, ok := httpx.ParseUUIDQuery(c, "application_id")
	if !ok {
		return
	}
	if applicationID != nil {
		filter.ApplicationID = applicationID
	}
	imageID, ok := httpx.ParseUUIDQuery(c, "image_id")
	if !ok {
		return
	}
	if imageID != nil {
		filter.ImageID = imageID
	}
	items, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, items)
}

// GetManifest godoc
// @Summary Get manifest
// @Tags Manifest
// @Produce json
// @Param id path string true "Manifest ID"
// @Success 200 {object} ManifestResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/manifests/{id} [get]
func (h *ManifestHandler) Get(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WriteData(c, http.StatusOK, item)
}

// GetManifestResources godoc
// @Summary Get manifest frozen resources
// @Tags Manifest
// @Produce json
// @Param id path string true "Manifest ID"
// @Success 200 {object} ManifestResourcesResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/manifests/{id}/resources [get]
func (h *ManifestHandler) GetResources(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetResources(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WriteData(c, http.StatusOK, item)
}

// DeleteManifest godoc
// @Summary Delete manifest
// @Tags Manifest
// @Param id path string true "Manifest ID"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/manifests/{id} [delete]
func (h *ManifestHandler) Delete(c *gin.Context) {
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

func writeManifestError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteNotFound(c, "not found")
	case errors.Is(err, manifestservice.ErrManifestImageApplicationMismatch),
		errors.Is(err, manifestservice.ErrManifestAppConfigMissing),
		errors.Is(err, manifestservice.ErrManifestWorkloadConfigMissing),
		errors.Is(err, manifestservice.ErrManifestRouteTargetInvalid),
		errors.Is(err, manifestservice.ErrManifestImageRepositoryMissing),
		errors.Is(err, manifestservice.ErrManifestImageNotDeployable),
		errors.Is(err, releasesupport.ErrDeployTargetBindingMissing),
		errors.Is(err, releasesupport.ErrDeployTargetBindingMalformed),
		errors.Is(err, releasesupport.ErrDeployTargetApplicationMetadataMissing),
		errors.Is(err, releasesupport.ErrDeployTargetApplicationMetadataMalformed),
		errors.Is(err, releasesupport.ErrDeployTargetProjectMetadataMissing),
		errors.Is(err, releasesupport.ErrDeployTargetProjectMetadataMalformed),
		errors.Is(err, releasesupport.ErrDeployTargetEnvironmentMetadataMissing),
		errors.Is(err, releasesupport.ErrDeployTargetEnvironmentMetadataMalformed),
		errors.Is(err, releasesupport.ErrDeployTargetClusterMetadataMissing),
		errors.Is(err, releasesupport.ErrDeployTargetClusterMetadataMalformed),
		errors.Is(err, releasesupport.ErrDeployTargetClusterNotReady),
		errors.Is(err, releasesupport.ErrDeployTargetClusterReadinessMalformed),
		errors.Is(err, releasesupport.ErrDeployTargetNamespaceInvalid),
		errors.Is(err, releasesupport.ErrDeployTargetClusterServerInvalid):
		httpx.WriteFailedPrecondition(c, http.StatusConflict, err.Error())
	default:
		httpx.WriteInternalError(c, err)
	}
}
