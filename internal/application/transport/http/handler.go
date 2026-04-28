package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/application/domain"
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type applicationService interface {
	Create(context.Context, *domain.Application) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*domain.Application, error)
	Update(context.Context, *domain.Application) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, domain.ListFilter) ([]domain.Application, error)
}

type Handler struct {
	svc applicationService
}

func NewHandler(svc applicationService) *Handler {
	return &Handler{svc: svc}
}

type CreateApplicationRequest struct {
	ProjectID   uuid.UUID          `json:"project_id"`
	Name        string             `json:"name"`
	RepoAddress string             `json:"repo_address"`
	Description string             `json:"description,omitempty"`
	Labels      []domain.LabelItem `json:"labels,omitempty"`
}

type UpdateApplicationRequest struct {
	ProjectID   uuid.UUID          `json:"project_id"`
	Name        string             `json:"name"`
	RepoAddress string             `json:"repo_address"`
	Description string             `json:"description,omitempty"`
	Labels      []domain.LabelItem `json:"labels,omitempty"`
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	app := rg.Group("/applications")
	app.GET("", h.List)
	app.GET("/:id", h.Get)
	app.POST("", h.Create)
	app.PUT("/:id", h.Update)
	app.DELETE("/:id", h.Delete)
}

// Create godoc
// @Summary 创建应用
// @Description 创建一个新的应用
// @Tags Application
// @Accept json
// @Produce json
// @Param data body CreateApplicationRequest true "Application Data"
// @Success 201 {object} httpx.DataResponse[domain.Application]
// @Router /api/v1/applications [post]
func (h *Handler) Create(c *gin.Context) {
	var req CreateApplicationRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	appRecord := &domain.Application{
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		RepoAddress: req.RepoAddress,
		Description: req.Description,
		Labels:      req.Labels,
	}
	appRecord.WithCreateDefault()
	_, err := h.svc.Create(c.Request.Context(), appRecord)
	if err != nil {
		if errors.Is(err, domain.ErrProjectReferenceNotFound) {
			httpx.WriteInvalidArgument(c, err.Error())
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WriteData(c, http.StatusCreated, appRecord)
}

// Get godoc
// @Summary 获取应用
// @Tags Application
// @Param id path string true "Application ID"
// @Success 200 {object} httpx.DataResponse[domain.Application]
// @Router /api/v1/applications/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	appRecord, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusOK, appRecord)
}

// Update godoc
// @Summary 更新应用
// @Tags Application
// @Param id path string true "Application ID"
// @Param data body UpdateApplicationRequest true "Application Data"
// @Success 204
// @Router /api/v1/applications/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	var req UpdateApplicationRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	appRecord := domain.Application{
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		RepoAddress: req.RepoAddress,
		Description: req.Description,
		Labels:      req.Labels,
	}
	appRecord.SetID(id)

	if err := h.svc.Update(c.Request.Context(), &appRecord); err != nil {
		if errors.Is(err, domain.ErrProjectReferenceNotFound) {
			httpx.WriteInvalidArgument(c, err.Error())
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}

	httpx.WriteNoContent(c)
}

// Delete godoc
// @Summary 删除应用
// @Tags Application
// @Param id path string true "Application ID"
// @Success 204
// @Router /api/v1/applications/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
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

// List godoc
// @Summary 获取应用列表
// @Tags Application
// @Success 200 {object} httpx.ListResponse[domain.Application]
// @Router /api/v1/applications [get]
func (h *Handler) List(c *gin.Context) {
	filter := domain.ListFilter{
		IncludeDeleted: httpx.IncludeDeleted(c),
		Name:           c.Query("name"),
		RepoAddress:    c.Query("repo_address"),
	}
	projectID, ok := httpx.ParseUUIDQuery(c, "project_id")
	if !ok {
		return
	}
	if projectID != nil {
		filter.ProjectID = projectID
	}

	apps, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, apps)
}
