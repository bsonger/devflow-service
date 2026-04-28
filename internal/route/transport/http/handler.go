package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/bsonger/devflow-service/internal/route/domain"
	route "github.com/bsonger/devflow-service/internal/route/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type routeService interface {
	Create(ctx context.Context, route *domain.Route) (uuid.UUID, error)
	Get(ctx context.Context, applicationId, id uuid.UUID) (*domain.Route, error)
	Update(ctx context.Context, route *domain.Route) error
	Delete(ctx context.Context, applicationId, id uuid.UUID) error
	List(ctx context.Context, filter RouteListFilter) ([]domain.Route, error)
	Validate(ctx context.Context, route *domain.Route) []string
}

type RouteListFilter = route.RouteListFilter

type Handler struct {
	routes routeService
}

func NewHandler(routes routeService) *Handler {
	return &Handler{routes: routes}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	routes := rg.Group("/routes")
	{
		routes.GET("", h.ListRoutes)
		routes.POST("", h.CreateRoute)
		routes.PATCH("/:route_id", h.UpdateRoute)
		routes.DELETE("/:route_id", h.DeleteRoute)
	}
	rg.POST("/routes:validate", h.ValidateRoute)
}

// Route handlers

// CreateRoute godoc
// @Summary Create application route
// @Tags Route
// @Accept json
// @Produce json
// @Param data body domain.RouteInput true "Route data"
// @Success 201 {object} httpx.DataResponse[domain.Route]
// @Router /api/v1/routes [post]
func (h *Handler) CreateRoute(c *gin.Context) {
	var req domain.RouteInput
	if !httpx.BindJSON(c, &req) {
		return
	}
	item := &domain.Route{
		ApplicationID: req.ApplicationID,
		EnvironmentID: req.EnvironmentID,
		Name:          req.Name,
		Host:          req.Host,
		Path:          req.Path,
		ServiceName:   req.ServiceName,
		ServicePort:   req.ServicePort,
	}
	item.WithCreateDefault()
	if _, err := h.routes.Create(c.Request.Context(), item); err != nil {
		httpx.WriteInvalidArgument(c, err.Error())
		return
	}
	httpx.WriteData(c, http.StatusCreated, item)
}

// ListRoutes godoc
// @Summary List application routes
// @Tags Route
// @Produce json
// @Param application_id query string true "Application ID"
// @Param environment_id query string true "Environment ID"
// @Param name query string false "Route name"
// @Param include_deleted query bool false "Include deleted items"
// @Param page query int false "Page"
// @Param page_size query int false "Page size"
// @Success 200 {object} httpx.ListResponse[domain.Route]
// @Router /api/v1/routes [get]
func (h *Handler) ListRoutes(c *gin.Context) {
	applicationId, ok := httpx.ParseUUIDQuery(c, "application_id")
	if !ok || applicationId == nil {
		if ok {
			httpx.WriteInvalidArgument(c, "application_id is required")
		}
		return
	}
	environmentId := c.Query("environment_id")
	if environmentId == "" {
		httpx.WriteInvalidArgument(c, "environment_id is required")
		return
	}
	filter := RouteListFilter{
		ApplicationID:  *applicationId,
		EnvironmentID:  environmentId,
		IncludeDeleted: httpx.IncludeDeleted(c),
		Name:           c.Query("name"),
	}
	items, err := h.routes.List(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, items)
}

// UpdateRoute godoc
// @Summary Update application route
// @Tags Route
// @Accept json
// @Produce json
// @Param route_id path string true "Route ID"
// @Param data body domain.RouteInput true "Route data"
// @Success 204
// @Router /api/v1/routes/{route_id} [patch]
func (h *Handler) UpdateRoute(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "route_id")
	if !ok {
		return
	}
	var req domain.RouteInput
	if !httpx.BindJSON(c, &req) {
		return
	}
	item := &domain.Route{
		ApplicationID: req.ApplicationID,
		EnvironmentID: req.EnvironmentID,
		Name:          req.Name,
		Host:          req.Host,
		Path:          req.Path,
		ServiceName:   req.ServiceName,
		ServicePort:   req.ServicePort,
	}
	item.SetID(id)
	if err := h.routes.Update(c.Request.Context(), item); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInvalidArgument(c, err.Error())
		return
	}
	httpx.WriteNoContent(c)
}

// DeleteRoute godoc
// @Summary Delete application route
// @Tags Route
// @Param route_id path string true "Route ID"
// @Success 204
// @Param application_id query string true "Application ID"
// @Router /api/v1/routes/{route_id} [delete]
func (h *Handler) DeleteRoute(c *gin.Context) {
	applicationId, ok := httpx.ParseUUIDQuery(c, "application_id")
	if !ok || applicationId == nil {
		if ok {
			httpx.WriteInvalidArgument(c, "invalid application_id")
		}
		return
	}
	id, ok := httpx.ParseUUIDParam(c, "route_id")
	if !ok {
		return
	}
	if err := h.routes.Delete(c.Request.Context(), *applicationId, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

// ValidateRoute godoc
// @Summary Validate application route
// @Tags Route
// @Accept json
// @Produce json
// @Param data body domain.RouteInput true "Route data"
// @Success 200 {object} httpx.DataResponse[domain.RouteValidationResult]
// @Router /api/v1/routes:validate [post]
func (h *Handler) ValidateRoute(c *gin.Context) {
	var req domain.RouteInput
	if !httpx.BindJSON(c, &req) {
		return
	}
	item := &domain.Route{
		ApplicationID: req.ApplicationID,
		EnvironmentID: req.EnvironmentID,
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
