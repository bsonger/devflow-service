package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/bsonger/devflow-service/internal/project/application"
	"github.com/bsonger/devflow-service/internal/project/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type projectService interface {
	Create(context.Context, *domain.Project) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*domain.Project, error)
	Update(context.Context, *domain.Project) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, application.ProjectListFilter) ([]domain.Project, error)
	ListApplications(context.Context, uuid.UUID) ([]domain.Application, error)
}

type Handler struct {
	svc projectService
}

type CreateProjectRequest struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Labels      []domain.LabelItem `json:"labels,omitempty"`
}

type UpdateProjectRequest = CreateProjectRequest

func NewHandler(svc projectService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	project := rg.Group("/projects")

	project.GET("", h.List)
	project.GET("/:id", h.Get)
	project.POST("", h.Create)
	project.PUT("/:id", h.Update)
	project.DELETE("/:id", h.Delete)
	project.GET("/:id/applications", h.ListApplications)
}

// Create godoc
// @Summary 创建项目
// @Description 创建一个新的项目
// @Tags Project
// @Accept json
// @Produce json
// @Param data body CreateProjectRequest true "Project Data"
// @Success 201 {object} httpx.DataResponse[domain.Project]
// @Router /api/v1/projects [post]
func (h *Handler) Create(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	project := &domain.Project{
		Name:        req.Name,
		Description: req.Description,
		Labels:      req.Labels,
	}
	project.WithCreateDefault()

	_, err := h.svc.Create(c.Request.Context(), project)
	if err != nil {
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	httpx.WriteData(c, http.StatusCreated, project)
}

// Get godoc
// @Summary 获取项目
// @Tags Project
// @Param id path string true "Project ID"
// @Success 200 {object} httpx.DataResponse[domain.Project]
// @Router /api/v1/projects/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	project, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	httpx.WriteData(c, http.StatusOK, project)
}

// Update godoc
// @Summary 更新项目
// @Tags Project
// @Param id path string true "Project ID"
// @Param data body UpdateProjectRequest true "Project Data"
// @Success 204
// @Router /api/v1/projects/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	project := domain.Project{
		Name:        req.Name,
		Description: req.Description,
		Labels:      req.Labels,
	}
	project.SetID(id)

	if err := h.svc.Update(c.Request.Context(), &project); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	httpx.WriteNoContent(c)
}

// Delete godoc
// @Summary 删除项目
// @Tags Project
// @Param id path string true "Project ID"
// @Success 204
// @Router /api/v1/projects/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	httpx.WriteNoContent(c)
}

// List godoc
// @Summary 获取项目列表
// @Tags Project
// @Success 200 {object} httpx.ListResponse[domain.Project]
// @Router /api/v1/projects [get]
func (h *Handler) List(c *gin.Context) {
	filter := application.ProjectListFilter{
		IncludeDeleted: httpx.IncludeDeleted(c),
		Name:           c.Query("name"),
	}

	projects, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	paging, err := httpx.ParsePagination(c)
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	total := len(projects)
	projects = httpx.PaginateSlice(projects, paging)
	httpx.WriteList(c, http.StatusOK, projects, paging, total)
}

// ListApplications godoc
// @Summary 获取项目下的应用列表
// @Tags Project
// @Param id path string true "Project ID"
// @Success 200 {object} httpx.ListResponse[domain.Application]
// @Router /api/v1/projects/{id}/applications [get]
func (h *Handler) ListApplications(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	applications, err := h.svc.ListApplications(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	paging, err := httpx.ParsePagination(c)
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	total := len(applications)
	applications = httpx.PaginateSlice(applications, paging)
	httpx.WriteList(c, http.StatusOK, applications, paging, total)
}
