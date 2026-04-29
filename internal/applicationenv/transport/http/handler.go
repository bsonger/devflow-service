package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/applicationenv/domain"
	applicationenvservice "github.com/bsonger/devflow-service/internal/applicationenv/service"
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type bindingService interface {
	Attach(context.Context, uuid.UUID, domain.BindingInput) (*domain.Binding, error)
	Get(context.Context, uuid.UUID, string) (*domain.Binding, error)
	List(context.Context, uuid.UUID) ([]applicationenvservice.BindingView, error)
	Delete(context.Context, uuid.UUID, string) error
	GetDetail(context.Context, uuid.UUID, string) (*applicationenvservice.BindingDetail, error)
}

type Handler struct {
	svc bindingService
}

func NewHandler(svc bindingService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	appEnvironments := rg.Group("/applications/:id/environments")
	appEnvironments.GET("", h.List)
	appEnvironments.GET("/:environment_id", h.Get)
	appEnvironments.POST("", h.Attach)
	appEnvironments.DELETE("/:environment_id", h.Delete)
}

// Attach
// @Summary Attach environment to application
// @Tags ApplicationEnvironment
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param data body domain.BindingInput true "Binding Data"
// @Success 201 {object} domain.Binding
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/applications/{id}/environments [post]
func (h *Handler) Attach(c *gin.Context) {
	applicationId, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	var req domain.BindingInput
	if !httpx.BindJSON(c, &req) {
		return
	}

	item, err := h.svc.Attach(c.Request.Context(), applicationId, req)
	if err != nil {
		writeBindingError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusCreated, item)
}

// List
// @Summary List application environment bindings
// @Tags ApplicationEnvironment
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} httpx.PaginatedResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/applications/{id}/environments [get]
func (h *Handler) List(c *gin.Context) {
	applicationId, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	items, err := h.svc.List(c.Request.Context(), applicationId)
	if err != nil {
		writeBindingError(c, err)
		return
	}

	httpx.WritePaginatedList(c, http.StatusOK, items)
}

// Get
// @Summary Get application environment binding detail
// @Tags ApplicationEnvironment
// @Produce json
// @Param id path string true "Application ID"
// @Param environment_id path string true "Environment ID"
// @Success 200 {object} applicationenvservice.BindingDetail
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/applications/{id}/environments/{environment_id} [get]
func (h *Handler) Get(c *gin.Context) {
	applicationId, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	item, err := h.svc.GetDetail(c.Request.Context(), applicationId, c.Param("environment_id"))
	if err != nil {
		writeBindingError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusOK, item)
}

// Delete
// @Summary Delete application environment binding
// @Tags ApplicationEnvironment
// @Param id path string true "Application ID"
// @Param environment_id path string true "Environment ID"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/applications/{id}/environments/{environment_id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	applicationId, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), applicationId, c.Param("environment_id")); err != nil {
		writeBindingError(c, err)
		return
	}

	httpx.WriteNoContent(c)
}

func writeBindingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteNotFound(c, "not found")
	case errors.Is(err, applicationenvservice.ErrApplicationReferenceNotFound),
		errors.Is(err, applicationenvservice.ErrEnvironmentReferenceNotFound),
		errors.Is(err, applicationenvservice.ErrEnvironmentIDRequired):
		httpx.WriteInvalidArgument(c, err.Error())
	default:
		httpx.WriteInternalError(c, err)
	}
}
