package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/appconfig/domain"
	appconfig "github.com/bsonger/devflow-service/internal/appconfig/service"
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type appConfigService interface {
	Create(ctx context.Context, cfg *domain.AppConfig) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.AppConfig, error)
	Update(ctx context.Context, cfg *domain.AppConfig) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter appconfig.AppConfigListFilter) ([]domain.AppConfig, error)
	Sync(ctx context.Context, id uuid.UUID) (*appconfig.AppConfigSyncResult, error)
}

type Handler struct {
	appConfigs appConfigService
}

func NewHandler(appConfigs appConfigService) *Handler {
	return &Handler{appConfigs: appConfigs}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	appConfigs := rg.Group("/app-configs")
	{
		appConfigs.GET("", h.ListAppConfigs)
		appConfigs.GET("/:id", h.GetAppConfig)
		appConfigs.POST("", h.CreateAppConfig)
		appConfigs.PUT("/:id", h.UpdateAppConfig)
		appConfigs.DELETE("/:id", h.DeleteAppConfig)
		appConfigs.POST("/:id/sync-from-repo", h.SyncAppConfig)
	}
}

// AppConfig handlers

// CreateAppConfig godoc
// @Summary Create app config
// @Tags AppConfig
// @Accept json
// @Produce json
// @Param data body domain.AppConfigInput true "AppConfig data"
// @Success 201 {object} httpx.DataResponse[domain.AppConfig]
// @Router /api/v1/app-configs [post]
func (h *Handler) CreateAppConfig(c *gin.Context) {
	var req domain.AppConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	item := &domain.AppConfig{
		ApplicationID: req.ApplicationID,
		EnvironmentID: req.EnvironmentID,
		Name:          req.Name,
		Description:   req.Description,
		Format:        req.Format,
		Data:          req.Data,
		MountPath:     req.MountPath,
		Labels:        req.Labels,
		SourcePath:    req.SourcePath,
	}
	item.WithCreateDefault()
	if _, err := h.appConfigs.Create(c.Request.Context(), item); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	httpx.WriteData(c, http.StatusCreated, item)
}

// GetAppConfig godoc
// @Summary Get app config
// @Tags AppConfig
// @Produce json
// @Param id path string true "AppConfig ID"
// @Success 200 {object} httpx.DataResponse[domain.AppConfig]
// @Router /api/v1/app-configs/{id} [get]
func (h *Handler) GetAppConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}
	item, err := h.appConfigs.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	httpx.WriteData(c, http.StatusOK, item)
}

// UpdateAppConfig godoc
// @Summary Update app config
// @Tags AppConfig
// @Accept json
// @Param id path string true "AppConfig ID"
// @Param data body domain.AppConfigInput true "AppConfig data"
// @Success 204
// @Router /api/v1/app-configs/{id} [put]
func (h *Handler) UpdateAppConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}
	var req domain.AppConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	item := &domain.AppConfig{
		ApplicationID: req.ApplicationID,
		EnvironmentID: req.EnvironmentID,
		Name:          req.Name,
		Description:   req.Description,
		Format:        req.Format,
		Data:          req.Data,
		MountPath:     req.MountPath,
		Labels:        req.Labels,
		SourcePath:    req.SourcePath,
	}
	item.SetID(id)
	if err := h.appConfigs.Update(c.Request.Context(), item); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	httpx.WriteNoContent(c)
}

// DeleteAppConfig godoc
// @Summary Delete app config
// @Tags AppConfig
// @Param id path string true "AppConfig ID"
// @Success 204
// @Router /api/v1/app-configs/{id} [delete]
func (h *Handler) DeleteAppConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}
	if err := h.appConfigs.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	httpx.WriteNoContent(c)
}

// ListAppConfigs godoc
// @Summary List app configs
// @Tags AppConfig
// @Produce json
// @Param application_id query string false "Application ID"
// @Param environment_id query string false "Environment ID"
// @Param name query string false "Name"
// @Param page query int false "Page"
// @Param page_size query int false "Page size"
// @Success 200 {object} httpx.ListResponse[domain.AppConfig]
// @Router /api/v1/app-configs [get]
func (h *Handler) ListAppConfigs(c *gin.Context) {
	var filter appconfig.AppConfigListFilter
	if appID := c.Query("application_id"); appID != "" {
		id, err := uuid.Parse(appID)
		if err != nil {
			httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid application_id", nil)
			return
		}
		filter.ApplicationID = &id
	}
	filter.EnvironmentID = c.Query("environment_id")
	filter.Name = c.Query("name")
	filter.IncludeDeleted = httpx.IncludeDeleted(c)
	items, err := h.appConfigs.List(c.Request.Context(), filter)
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

// SyncAppConfig godoc
// @Summary Sync app config from fixed config repo
// @Tags AppConfig
// @Produce json
// @Param id path string true "AppConfig ID"
// @Success 200 {object} httpx.DataResponse[domain.AppConfigRevision]
// @Router /api/v1/app-configs/{id}/sync-from-repo [post]
func (h *Handler) SyncAppConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}
	result, err := h.appConfigs.Sync(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
		case errors.Is(err, appconfig.ErrConfigSourceNotFound), errors.Is(err, appconfig.ErrConfigRepositoryUnavailable), errors.Is(err, appconfig.ErrConfigRepositorySyncFailed):
			httpx.WriteError(c, http.StatusFailedDependency, "failed_precondition", err.Error(), nil)
		default:
			httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		}
		return
	}
	httpx.WriteData(c, http.StatusOK, result.Revision)
}

