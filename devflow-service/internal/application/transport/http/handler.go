package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	appsvc "github.com/bsonger/devflow-service/internal/application/application"
	appdomain "github.com/bsonger/devflow-service/internal/application/domain"
	platformhttpx "github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type applicationService interface {
	Create(context.Context, *appdomain.Application) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*appdomain.Application, error)
	Update(context.Context, *appdomain.Application) error
	Delete(context.Context, uuid.UUID) error
	UpdateActiveImage(context.Context, uuid.UUID, uuid.UUID) error
	List(context.Context, appsvc.ListFilter) ([]appdomain.Application, error)
}

type Handler struct {
	svc applicationService
}

func NewHandler(svc applicationService) *Handler {
	return &Handler{svc: svc}
}

type CreateApplicationRequest struct {
	ProjectID   uuid.UUID             `json:"project_id"`
	Name        string                `json:"name"`
	RepoAddress string                `json:"repo_address"`
	Description string                `json:"description,omitempty"`
	Labels      []appdomain.LabelItem `json:"labels,omitempty"`
}

type UpdateApplicationRequest struct {
	ProjectID     uuid.UUID             `json:"project_id"`
	Name          string                `json:"name"`
	RepoAddress   string                `json:"repo_address"`
	Description   string                `json:"description,omitempty"`
	ActiveImageID *uuid.UUID            `json:"active_image_id,omitempty"`
	Labels        []appdomain.LabelItem `json:"labels,omitempty"`
}

type UpdateActiveImageRequest struct {
	ImageID string `json:"image_id" binding:"required"`
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	app := rg.Group("/applications")
	app.GET("", h.List)
	app.GET("/:id", h.Get)
	app.POST("", h.Create)
	app.PUT("/:id", h.Update)
	app.DELETE("/:id", h.Delete)
	app.PATCH("/:id/active_image", h.UpdateActiveImage)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	application := &appdomain.Application{
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		RepoAddress: req.RepoAddress,
		Description: req.Description,
		Labels:      req.Labels,
	}
	application.WithCreateDefault()
	_, err := h.svc.Create(c.Request.Context(), application)
	if err != nil {
		if errors.Is(err, appsvc.ErrProjectReferenceNotFound) {
			platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
			return
		}
		platformhttpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	platformhttpx.WriteData(c, http.StatusCreated, application)
}

func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	application, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			platformhttpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		platformhttpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	platformhttpx.WriteData(c, http.StatusOK, application)
}

func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	var req UpdateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	application := appdomain.Application{
		ProjectID:     req.ProjectID,
		Name:          req.Name,
		RepoAddress:   req.RepoAddress,
		Description:   req.Description,
		ActiveImageID: req.ActiveImageID,
		Labels:        req.Labels,
	}
	application.SetID(id)

	if err := h.svc.Update(c.Request.Context(), &application); err != nil {
		if errors.Is(err, appsvc.ErrProjectReferenceNotFound) {
			platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
			return
		}
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

func (h *Handler) UpdateActiveImage(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	var req UpdateActiveImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	imageID, err := uuid.Parse(req.ImageID)
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid image_id", nil)
		return
	}

	if err := h.svc.UpdateActiveImage(c.Request.Context(), appID, imageID); err != nil {
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
	filter := appsvc.ListFilter{
		IncludeDeleted: platformhttpx.IncludeDeleted(c),
		Name:           c.Query("name"),
		RepoAddress:    c.Query("repo_address"),
	}
	if projectID := c.Query("project_id"); projectID != "" {
		id, err := uuid.Parse(projectID)
		if err != nil {
			platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid project_id", nil)
			return
		}
		filter.ProjectID = &id
	}

	apps, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		platformhttpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	paging, err := platformhttpx.ParsePagination(c)
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	total := len(apps)
	apps = platformhttpx.PaginateSlice(apps, paging)
	platformhttpx.WriteList(c, http.StatusOK, apps, paging, total)
}
