package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	httpx "github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/app"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var EnvironmentRouteApi = NewEnvironmentHandler()

type environmentService interface {
	Create(ctx context.Context, environment *domain.Environment) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Environment, error)
	Update(ctx context.Context, environment *domain.Environment) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter app.EnvironmentListFilter) ([]domain.Environment, error)
}

type EnvironmentHandler struct {
	svc environmentService
}

type CreateEnvironmentRequest struct {
	Name        string             `json:"name" binding:"required"`
	ClusterID   uuid.UUID          `json:"cluster_id" binding:"required"`
	Description string             `json:"description,omitempty"`
	Labels      []domain.LabelItem `json:"labels,omitempty"`
}

type UpdateEnvironmentRequest = CreateEnvironmentRequest

func NewEnvironmentHandler() *EnvironmentHandler {
	return &EnvironmentHandler{svc: app.EnvironmentService}
}

// Create
// @Summary 创建环境
// @Description 创建一个新的环境
// @Tags Environment
// @Accept json
// @Produce json
// @Param data body api.CreateEnvironmentRequest true "Environment Data"
// @Success 201 {object} httpx.DataResponse[domain.Environment]
// @Router /api/v1/environments [post]
func (h *EnvironmentHandler) Create(c *gin.Context) {
	var req CreateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
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

// Get
// @Summary 获取环境
// @Tags Environment
// @Param id path string true "Environment ID"
// @Success 200 {object} httpx.DataResponse[domain.Environment]
// @Router /api/v1/environments/{id} [get]
func (h *EnvironmentHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	environment, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		writeEnvironmentError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusOK, environment)
}

// Update
// @Summary 更新环境
// @Tags Environment
// @Param id path string true "Environment ID"
// @Param data body api.UpdateEnvironmentRequest true "Environment Data"
// @Success 204
// @Router /api/v1/environments/{id} [put]
func (h *EnvironmentHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	var req UpdateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
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

// Delete
// @Summary 删除环境
// @Tags Environment
// @Param id path string true "Environment ID"
// @Success 204
// @Router /api/v1/environments/{id} [delete]
func (h *EnvironmentHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeEnvironmentError(c, err)
		return
	}

	httpx.WriteNoContent(c)
}

// List
// @Summary 获取环境列表
// @Tags Environment
// @Success 200 {object} httpx.ListResponse[domain.Environment]
// @Router /api/v1/environments [get]
func (h *EnvironmentHandler) List(c *gin.Context) {
	filter := app.EnvironmentListFilter{
		IncludeDeleted: httpx.IncludeDeleted(c),
		Name:           c.Query("name"),
	}
	if clusterID := c.Query("cluster_id"); clusterID != "" {
		id, err := uuid.Parse(clusterID)
		if err != nil {
			httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid cluster_id", nil)
			return
		}
		filter.ClusterID = &id
	}

	environments, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		writeEnvironmentError(c, err)
		return
	}

	paging, err := httpx.ParsePagination(c)
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	total := len(environments)
	environments = httpx.PaginateSlice(environments, paging)
	httpx.WriteList(c, http.StatusOK, environments, paging, total)
}

func writeEnvironmentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
	case errors.Is(err, app.ErrEnvironmentConflict):
		httpx.WriteError(c, http.StatusConflict, "conflict", err.Error(), nil)
	case errors.Is(err, app.ErrEnvironmentNameRequired),
		errors.Is(err, app.ErrEnvironmentClusterRequired),
		errors.Is(err, app.ErrClusterReferenceNotFound):
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
	default:
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
	}
}
