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

var ApplicationRouteApi = NewApplicationHandler()

type applicationService interface {
	Create(ctx context.Context, application *domain.Application) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Application, error)
	Update(ctx context.Context, application *domain.Application) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateActiveImage(ctx context.Context, appID, imageID uuid.UUID) error
	List(ctx context.Context, filter app.ApplicationListFilter) ([]domain.Application, error)
}

type ApplicationHandler struct {
	svc applicationService
}

func NewApplicationHandler() *ApplicationHandler {
	return &ApplicationHandler{
		svc: app.ApplicationService,
	}
}

type CreateApplicationRequest struct {
	ProjectID   uuid.UUID          `json:"project_id"`
	Name        string             `json:"name"`
	RepoAddress string             `json:"repo_address"`
	Description string             `json:"description,omitempty"`
	Labels      []domain.LabelItem `json:"labels,omitempty"`
}

type UpdateApplicationRequest struct {
	ProjectID     uuid.UUID          `json:"project_id"`
	Name          string             `json:"name"`
	RepoAddress   string             `json:"repo_address"`
	Description   string             `json:"description,omitempty"`
	ActiveImageID *uuid.UUID         `json:"active_image_id,omitempty"`
	Labels        []domain.LabelItem `json:"labels,omitempty"`
}

type UpdateActiveImageRequest struct {
	ImageID string `json:"image_id" binding:"required"`
}

// Create
// @Summary 创建应用
// @Description 创建一个新的应用
// @Tags Application
// @Accept json
// @Produce json
// @Param data body api.CreateApplicationRequest true "Application Data"
// @Success 201 {object} httpx.DataResponse[domain.Application]
// @Router /api/v1/applications [post]
func (h *ApplicationHandler) Create(c *gin.Context) {
	var req CreateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	application := &domain.Application{
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		RepoAddress: req.RepoAddress,
		Description: req.Description,
		Labels:      req.Labels,
	}
	application.WithCreateDefault()
	_, err := h.svc.Create(c.Request.Context(), application)
	if err != nil {
		if errors.Is(err, app.ErrProjectReferenceNotFound) {
			httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	httpx.WriteData(c, http.StatusCreated, application)
}

// Get
// @Summary	获取应用
// @Tags		Application
// @Param		id	path		string	true	"Application ID"
// @Success	200	{object}	httpx.DataResponse[domain.Application]
// @Router		/api/v1/applications/{id} [get]
func (h *ApplicationHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	application, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	httpx.WriteData(c, http.StatusOK, application)
}

// Update
// @Summary	更新应用
// @Tags		Application
// @Param		id		path		string				true	"Application ID"
// @Param		data	body		api.UpdateApplicationRequest	true	"Application Data"
// @Success	204
// @Router		/api/v1/applications/{id} [put]
func (h *ApplicationHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	var req UpdateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	application := domain.Application{
		ProjectID:     req.ProjectID,
		Name:          req.Name,
		RepoAddress:   req.RepoAddress,
		Description:   req.Description,
		ActiveImageID: req.ActiveImageID,
		Labels:        req.Labels,
	}
	application.SetID(id)

	if err := h.svc.Update(c.Request.Context(), &application); err != nil {
		if errors.Is(err, app.ErrProjectReferenceNotFound) {
			httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
			return
		}
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
// @Summary	删除应用
// @Tags		Application
// @Param		id	path		string	true	"Application ID"
// @Success	204
// @Router		/api/v1/applications/{id} [delete]
func (h *ApplicationHandler) Delete(c *gin.Context) {
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

// UpdateActiveImage
// @Summary	更新应用的 Active Image
// @Tags		Application
// @Param		id	path		string	true	"Application ID"
// @Param		data	body		UpdateActiveImageRequest	true	"Active Image Data"
// @Success	204
// @Router		/api/v1/applications/{id}/active_image [patch]
func (h *ApplicationHandler) UpdateActiveImage(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	var req UpdateActiveImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	imageID, err := uuid.Parse(req.ImageID)
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid image_id", nil)
		return
	}

	if err := h.svc.UpdateActiveImage(c.Request.Context(), appID, imageID); err != nil {
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
// @Summary 获取应用列表
// @Tags    Application
// @Success 200 {object} httpx.ListResponse[domain.Application]
// @Router  /api/v1/applications [get]
func (h *ApplicationHandler) List(c *gin.Context) {
	filter := app.ApplicationListFilter{
		IncludeDeleted: httpx.IncludeDeleted(c),
		Name:           c.Query("name"),
		RepoAddress:    c.Query("repo_address"),
	}
	if projectID := c.Query("project_id"); projectID != "" {
		id, err := uuid.Parse(projectID)
		if err != nil {
			httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid project_id", nil)
			return
		}
		filter.ProjectID = &id
	}

	apps, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}

	paging, err := httpx.ParsePagination(c)
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	total := len(apps)
	apps = httpx.PaginateSlice(apps, paging)
	httpx.WriteList(c, http.StatusOK, apps, paging, total)
}
