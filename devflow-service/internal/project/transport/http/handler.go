package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	platformhttpx "github.com/bsonger/devflow-service/internal/platform/httpx"
	projectapp "github.com/bsonger/devflow-service/internal/project/application"
	projectdomain "github.com/bsonger/devflow-service/internal/project/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type projectService interface {
	Create(context.Context, *projectdomain.Project) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*projectdomain.Project, error)
	Update(context.Context, *projectdomain.Project) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, projectapp.ProjectListFilter) ([]projectdomain.Project, error)
	ListApplications(context.Context, uuid.UUID) ([]projectdomain.Application, error)
}

type Handler struct {
	svc projectService
}

type CreateProjectRequest struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description,omitempty"`
	Labels      []projectdomain.LabelItem `json:"labels,omitempty"`
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

func (h *Handler) Create(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	project := &projectdomain.Project{
		Name:        req.Name,
		Description: req.Description,
		Labels:      req.Labels,
	}
	project.WithCreateDefault()

	_, err := h.svc.Create(c.Request.Context(), project)
	if err != nil {
		platformhttpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	platformhttpx.WriteData(c, http.StatusCreated, project)
}

func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	project, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			platformhttpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		platformhttpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	platformhttpx.WriteData(c, http.StatusOK, project)
}

func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	project := projectdomain.Project{
		Name:        req.Name,
		Description: req.Description,
		Labels:      req.Labels,
	}
	project.SetID(id)

	if err := h.svc.Update(c.Request.Context(), &project); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			platformhttpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		platformhttpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	platformhttpx.WriteNoContent(c)
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			platformhttpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		platformhttpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	platformhttpx.WriteNoContent(c)
}

func (h *Handler) List(c *gin.Context) {
	filter := projectapp.ProjectListFilter{
		IncludeDeleted: platformhttpx.IncludeDeleted(c),
		Name:           c.Query("name"),
	}

	projects, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		platformhttpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	paging, err := platformhttpx.ParsePagination(c)
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	total := len(projects)
	projects = platformhttpx.PaginateSlice(projects, paging)
	platformhttpx.WriteList(c, http.StatusOK, projects, paging, total)
}

func (h *Handler) ListApplications(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	applications, err := h.svc.ListApplications(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			platformhttpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		platformhttpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	paging, err := platformhttpx.ParsePagination(c)
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	total := len(applications)
	applications = platformhttpx.PaginateSlice(applications, paging)
	platformhttpx.WriteList(c, http.StatusOK, applications, paging, total)
}
