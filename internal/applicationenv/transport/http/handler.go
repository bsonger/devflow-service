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

func (h *Handler) Attach(c *gin.Context) {
	applicationID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	var req domain.BindingInput
	if !httpx.BindJSON(c, &req) {
		return
	}

	item, err := h.svc.Attach(c.Request.Context(), applicationID, req)
	if err != nil {
		writeBindingError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusCreated, item)
}

func (h *Handler) List(c *gin.Context) {
	applicationID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	items, err := h.svc.List(c.Request.Context(), applicationID)
	if err != nil {
		writeBindingError(c, err)
		return
	}

	httpx.WritePaginatedList(c, http.StatusOK, items)
}

func (h *Handler) Get(c *gin.Context) {
	applicationID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	item, err := h.svc.GetDetail(c.Request.Context(), applicationID, c.Param("environment_id"))
	if err != nil {
		writeBindingError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusOK, item)
}

func (h *Handler) Delete(c *gin.Context) {
	applicationID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), applicationID, c.Param("environment_id")); err != nil {
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
