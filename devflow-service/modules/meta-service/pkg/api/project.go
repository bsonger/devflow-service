package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/modules/meta-service/pkg/app"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/domain"
	"github.com/bsonger/devflow-service/shared/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var ProjectRouteApi = NewProjectHandler()

type projectService interface {
	Create(ctx context.Context, project *domain.Project) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Project, error)
	Update(ctx context.Context, project *domain.Project) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter app.ProjectListFilter) ([]domain.Project, error)
	ListApplications(ctx context.Context, projectID uuid.UUID) ([]domain.Application, error)
}

type ProjectHandler struct {
	svc projectService
}

type CreateProjectRequest struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Labels      []domain.LabelItem `json:"labels,omitempty"`
}

type UpdateProjectRequest = CreateProjectRequest

func NewProjectHandler() *ProjectHandler {
	return &ProjectHandler{svc: app.ProjectService}
}

// Create
// @Summary 创建项目
// @Description 创建一个新的项目
// @Tags Project
// @Accept json
// @Produce json
// @Param data body api.CreateProjectRequest true "Project Data"
// @Success 201 {object} httpx.DataResponse[domain.Project]
// @Router /api/v1/projects [post]
func (h *ProjectHandler) Create(c *gin.Context) {
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

// Get
// @Summary 获取项目
// @Tags Project
// @Param id path string true "Project ID"
// @Success 200 {object} httpx.DataResponse[domain.Project]
// @Router /api/v1/projects/{id} [get]
func (h *ProjectHandler) Get(c *gin.Context) {
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

// Update
// @Summary 更新项目
// @Tags Project
// @Param id path string true "Project ID"
// @Param data body api.UpdateProjectRequest true "Project Data"
// @Success 204
// @Router /api/v1/projects/{id} [put]
func (h *ProjectHandler) Update(c *gin.Context) {
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

// Delete
// @Summary 删除项目
// @Tags Project
// @Param id path string true "Project ID"
// @Success 204
// @Router /api/v1/projects/{id} [delete]
func (h *ProjectHandler) Delete(c *gin.Context) {
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

// List
// @Summary 获取项目列表
// @Tags Project
// @Success 200 {object} httpx.ListResponse[domain.Project]
// @Router /api/v1/projects [get]
func (h *ProjectHandler) List(c *gin.Context) {
	filter := app.ProjectListFilter{
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

// ListApplications
// @Summary 获取项目下的应用列表
// @Tags Project
// @Param id path string true "Project ID"
// @Success 200 {object} httpx.ListResponse[domain.Application]
// @Router /api/v1/projects/{id}/applications [get]
func (h *ProjectHandler) ListApplications(c *gin.Context) {
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
