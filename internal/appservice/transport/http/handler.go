package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/appservice/domain"
	appservice "github.com/bsonger/devflow-service/internal/appservice/service"
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type serviceService interface {
	Create(ctx context.Context, service *domain.Service) (uuid.UUID, error)
	Get(ctx context.Context, applicationID, id uuid.UUID) (*domain.Service, error)
	Update(ctx context.Context, service *domain.Service) error
	Delete(ctx context.Context, applicationID, id uuid.UUID) error
	List(ctx context.Context, filter ServiceListFilter) ([]domain.Service, error)
}

type ServiceListFilter = appservice.ServiceListFilter

type Handler struct {
	services serviceService
}

func NewHandler(services serviceService) *Handler {
	return &Handler{services: services}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	services := rg.Group("/applications/:id/services")
	{
		services.GET("", h.ListServices)
		services.POST("", h.CreateService)
		services.PATCH("/:service_id", h.UpdateService)
		services.DELETE("/:service_id", h.DeleteService)
	}
}

// Service handlers

// CreateService godoc
// @Summary Create application service
// @Tags Service
// @Accept json
// @Produce json
// @Param application_id path string true "Application ID"
// @Param data body domain.ServiceInput true "Service data"
// @Success 201 {object} httpx.DataResponse[domain.Service]
// @Router /api/v1/applications/{application_id}/services [post]
func (h *Handler) CreateService(c *gin.Context) {
	applicationID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	var req domain.ServiceInput
	if !httpx.BindJSON(c, &req) {
		return
	}
	item := &domain.Service{ApplicationID: applicationID, Name: req.Name, Ports: req.Ports}
	item.WithCreateDefault()
	if _, err := h.services.Create(c.Request.Context(), item); err != nil {
		httpx.WriteInvalidArgument(c, err.Error())
		return
	}
	httpx.WriteData(c, http.StatusCreated, item)
}

// ListServices godoc
// @Summary List application services
// @Tags Service
// @Produce json
// @Param application_id path string true "Application ID"
// @Param name query string false "Service name"
// @Param include_deleted query bool false "Include deleted items"
// @Param page query int false "Page"
// @Param page_size query int false "Page size"
// @Success 200 {object} httpx.ListResponse[domain.Service]
// @Router /api/v1/applications/{application_id}/services [get]
func (h *Handler) ListServices(c *gin.Context) {
	applicationID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	items, err := h.services.List(c.Request.Context(), ServiceListFilter{
		ApplicationID:  applicationID,
		IncludeDeleted: httpx.IncludeDeleted(c),
		Name:           c.Query("name"),
	})
	if err != nil {
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, items)
}

// UpdateService godoc
// @Summary Update application service
// @Tags Service
// @Accept json
// @Produce json
// @Param application_id path string true "Application ID"
// @Param service_id path string true "Service ID"
// @Param data body domain.ServiceInput true "Service data"
// @Success 204
// @Router /api/v1/applications/{application_id}/services/{service_id} [patch]
func (h *Handler) UpdateService(c *gin.Context) {
	applicationID, id, ok := parseApplicationAndResourceID(c, "service_id")
	if !ok {
		return
	}
	var req domain.ServiceInput
	if !httpx.BindJSON(c, &req) {
		return
	}
	item := &domain.Service{ApplicationID: applicationID, Name: req.Name, Ports: req.Ports}
	item.SetID(id)
	if err := h.services.Update(c.Request.Context(), item); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInvalidArgument(c, err.Error())
		return
	}
	httpx.WriteNoContent(c)
}

// DeleteService godoc
// @Summary Delete application service
// @Tags Service
// @Param application_id path string true "Application ID"
// @Param service_id path string true "Service ID"
// @Success 204
// @Router /api/v1/applications/{application_id}/services/{service_id} [delete]
func (h *Handler) DeleteService(c *gin.Context) {
	applicationID, id, ok := parseApplicationAndResourceID(c, "service_id")
	if !ok {
		return
	}
	if err := h.services.Delete(c.Request.Context(), applicationID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func parseApplicationAndResourceID(c *gin.Context, key string) (uuid.UUID, uuid.UUID, bool) {
	applicationID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, ok := httpx.ParseUUIDParam(c, key)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	return applicationID, id, true
}
