package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/network/domain"
	networkservice "github.com/bsonger/devflow-service/internal/network/service"
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

type routeService interface {
	Create(ctx context.Context, route *domain.Route) (uuid.UUID, error)
	Get(ctx context.Context, applicationID, id uuid.UUID) (*domain.Route, error)
	Update(ctx context.Context, route *domain.Route) error
	Delete(ctx context.Context, applicationID, id uuid.UUID) error
	List(ctx context.Context, filter RouteListFilter) ([]domain.Route, error)
	Validate(ctx context.Context, route *domain.Route) []string
}

type ServiceListFilter = networkservice.ServiceListFilter
type RouteListFilter = networkservice.RouteListFilter

type Handler struct {
	services serviceService
	routes   routeService
}

func NewHandler(services serviceService, routes routeService) *Handler {
	return &Handler{services: services, routes: routes}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	services := rg.Group("/applications/:application_id/services")
	{
		services.GET("", h.ListServices)
		services.POST("", h.CreateService)
		services.PATCH("/:service_id", h.UpdateService)
		services.DELETE("/:service_id", h.DeleteService)
	}
	routes := rg.Group("/applications/:application_id/routes")
	{
		routes.GET("", h.ListRoutes)
		routes.POST("", h.CreateRoute)
		routes.PATCH("/:route_id", h.UpdateRoute)
		routes.DELETE("/:route_id", h.DeleteRoute)
	}
	rg.POST("/applications/:application_id/routes:validate", h.ValidateRoute)
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
	applicationID, err := uuid.Parse(c.Param("application_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid application id", nil)
		return
	}
	var req domain.ServiceInput
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	item := &domain.Service{ApplicationID: applicationID, Name: req.Name, Ports: req.Ports}
	item.WithCreateDefault()
	if _, err := h.services.Create(c.Request.Context(), item); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
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
	applicationID, err := uuid.Parse(c.Param("application_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid application id", nil)
		return
	}
	items, err := h.services.List(c.Request.Context(), ServiceListFilter{
		ApplicationID:  applicationID,
		IncludeDeleted: httpx.IncludeDeleted(c),
		Name:           c.Query("name"),
	})
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
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	item := &domain.Service{ApplicationID: applicationID, Name: req.Name, Ports: req.Ports}
	item.SetID(id)
	if err := h.services.Update(c.Request.Context(), item); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
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
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	httpx.WriteNoContent(c)
}

// Route handlers

// CreateRoute godoc
// @Summary Create application route
// @Tags Route
// @Accept json
// @Produce json
// @Param application_id path string true "Application ID"
// @Param data body domain.RouteInput true "Route data"
// @Success 201 {object} httpx.DataResponse[domain.Route]
// @Router /api/v1/applications/{application_id}/routes [post]
func (h *Handler) CreateRoute(c *gin.Context) {
	applicationID, err := uuid.Parse(c.Param("application_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid application id", nil)
		return
	}
	var req domain.RouteInput
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	item := &domain.Route{
		ApplicationID: applicationID,
		Name:          req.Name,
		Host:          req.Host,
		Path:          req.Path,
		ServiceName:   req.ServiceName,
		ServicePort:   req.ServicePort,
	}
	item.WithCreateDefault()
	if _, err := h.routes.Create(c.Request.Context(), item); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	httpx.WriteData(c, http.StatusCreated, item)
}

// ListRoutes godoc
// @Summary List application routes
// @Tags Route
// @Produce json
// @Param application_id path string true "Application ID"
// @Param name query string false "Route name"
// @Param include_deleted query bool false "Include deleted items"
// @Param page query int false "Page"
// @Param page_size query int false "Page size"
// @Success 200 {object} httpx.ListResponse[domain.Route]
// @Router /api/v1/applications/{application_id}/routes [get]
func (h *Handler) ListRoutes(c *gin.Context) {
	applicationID, err := uuid.Parse(c.Param("application_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid application id", nil)
		return
	}
	items, err := h.routes.List(c.Request.Context(), RouteListFilter{
		ApplicationID:  applicationID,
		IncludeDeleted: httpx.IncludeDeleted(c),
		Name:           c.Query("name"),
	})
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

// UpdateRoute godoc
// @Summary Update application route
// @Tags Route
// @Accept json
// @Produce json
// @Param application_id path string true "Application ID"
// @Param route_id path string true "Route ID"
// @Param data body domain.RouteInput true "Route data"
// @Success 204
// @Router /api/v1/applications/{application_id}/routes/{route_id} [patch]
func (h *Handler) UpdateRoute(c *gin.Context) {
	applicationID, id, ok := parseApplicationAndResourceID(c, "route_id")
	if !ok {
		return
	}
	var req domain.RouteInput
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	item := &domain.Route{
		ApplicationID: applicationID,
		Name:          req.Name,
		Host:          req.Host,
		Path:          req.Path,
		ServiceName:   req.ServiceName,
		ServicePort:   req.ServicePort,
	}
	item.SetID(id)
	if err := h.routes.Update(c.Request.Context(), item); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	httpx.WriteNoContent(c)
}

// DeleteRoute godoc
// @Summary Delete application route
// @Tags Route
// @Param application_id path string true "Application ID"
// @Param route_id path string true "Route ID"
// @Success 204
// @Router /api/v1/applications/{application_id}/routes/{route_id} [delete]
func (h *Handler) DeleteRoute(c *gin.Context) {
	applicationID, id, ok := parseApplicationAndResourceID(c, "route_id")
	if !ok {
		return
	}
	if err := h.routes.Delete(c.Request.Context(), applicationID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	httpx.WriteNoContent(c)
}

// ValidateRoute godoc
// @Summary Validate application route
// @Tags Route
// @Accept json
// @Produce json
// @Param application_id path string true "Application ID"
// @Param data body domain.RouteInput true "Route data"
// @Success 200 {object} httpx.DataResponse[domain.RouteValidationResult]
// @Router /api/v1/applications/{application_id}/routes:validate [post]
func (h *Handler) ValidateRoute(c *gin.Context) {
	applicationID, err := uuid.Parse(c.Param("application_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid application id", nil)
		return
	}
	var req domain.RouteInput
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	item := &domain.Route{
		ApplicationID: applicationID,
		Name:          req.Name,
		Host:          req.Host,
		Path:          req.Path,
		ServiceName:   req.ServiceName,
		ServicePort:   req.ServicePort,
	}
	errs := h.routes.Validate(c.Request.Context(), item)
	httpx.WriteData(c, http.StatusOK, domain.RouteValidationResult{
		Valid:  len(errs) == 0,
		Errors: errs,
	})
}

func parseApplicationAndResourceID(c *gin.Context, key string) (uuid.UUID, uuid.UUID, bool) {
	applicationID, err := uuid.Parse(c.Param("application_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid application id", nil)
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param(key))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid resource id", nil)
		return uuid.Nil, uuid.Nil, false
	}
	return applicationID, id, true
}
