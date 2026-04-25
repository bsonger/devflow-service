package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/environment/domain"
	"github.com/bsonger/devflow-service/internal/environment/service"
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type environmentService interface {
	Create(context.Context, *domain.Environment) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*domain.Environment, error)
	Update(context.Context, *domain.Environment) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, service.ListFilter) ([]domain.Environment, error)
}

type Handler struct {
	svc environmentService
}

type CreateEnvironmentRequest struct {
	Name        string             `json:"name" binding:"required"`
	ClusterID   uuid.UUID          `json:"cluster_id" binding:"required"`
	Description string             `json:"description,omitempty"`
	Labels      []domain.LabelItem `json:"labels,omitempty"`
}

type UpdateEnvironmentRequest = CreateEnvironmentRequest

func NewHandler(svc environmentService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	environment := rg.Group("/environments")
	environment.GET("", h.List)
	environment.GET("/:id", h.Get)
	environment.POST("", h.Create)
	environment.PUT("/:id", h.Update)
	environment.DELETE("/:id", h.Delete)
}

// Create godoc
// @Summary 创建环境
// @Tags Environment
// @Accept json
// @Produce json
// @Param data body CreateEnvironmentRequest true "Environment Data"
// @Success 201 {object} httpx.DataResponse[domain.Environment]
// @Router /api/v1/environments [post]
func (h *Handler) Create(c *gin.Context) {
	var req CreateEnvironmentRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	environment := &domain.Environment{
		Name:        req.Name,
		ClusterID:   req.ClusterID,
		Description: req.Description,
		Labels:      req.Labels,
	}
	environment.WithCreateDefault()

	_, err := h.svc.Create(c.Request.Context(), environment)
	if err != nil {
		writeEnvironmentError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusCreated, environment)
}

// Get godoc
// @Summary 获取环境
// @Tags Environment
// @Param id path string true "Environment ID"
// @Success 200 {object} httpx.DataResponse[domain.Environment]
// @Router /api/v1/environments/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	environment, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		writeEnvironmentError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusOK, environment)
}

// Update godoc
// @Summary 更新环境
// @Tags Environment
// @Param id path string true "Environment ID"
// @Param data body UpdateEnvironmentRequest true "Environment Data"
// @Success 204
// @Router /api/v1/environments/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	var req UpdateEnvironmentRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	environment := domain.Environment{
		Name:        req.Name,
		ClusterID:   req.ClusterID,
		Description: req.Description,
		Labels:      req.Labels,
	}
	environment.SetID(id)

	if err := h.svc.Update(c.Request.Context(), &environment); err != nil {
		writeEnvironmentError(c, err)
		return
	}

	httpx.WriteNoContent(c)
}

// Delete godoc
// @Summary 删除环境
// @Tags Environment
// @Param id path string true "Environment ID"
// @Success 204
// @Router /api/v1/environments/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeEnvironmentError(c, err)
		return
	}

	httpx.WriteNoContent(c)
}

// List godoc
// @Summary 获取环境列表
// @Tags Environment
// @Success 200 {object} httpx.ListResponse[domain.Environment]
// @Router /api/v1/environments [get]
func (h *Handler) List(c *gin.Context) {
	filter := service.ListFilter{
		IncludeDeleted: httpx.IncludeDeleted(c),
		Name:           c.Query("name"),
	}
	clusterID, ok := httpx.ParseUUIDQuery(c, "cluster_id")
	if !ok {
		return
	}
	if clusterID != nil {
		filter.ClusterID = clusterID
	}

	environments, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		writeEnvironmentError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, environments)
}

func writeEnvironmentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteNotFound(c, "not found")
	case errors.Is(err, service.ErrEnvironmentConflict):
		httpx.WriteConflict(c, err.Error())
	case errors.Is(err, service.ErrEnvironmentNameRequired),
		errors.Is(err, service.ErrEnvironmentClusterRequired),
		errors.Is(err, service.ErrClusterReferenceNotFound):
		httpx.WriteInvalidArgument(c, err.Error())
	default:
		httpx.WriteInternalError(c, err)
	}
}
