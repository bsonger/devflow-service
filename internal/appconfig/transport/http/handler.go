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
	if !httpx.BindJSON(c, &req) {
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
		httpx.WriteInvalidArgument(c, err.Error())
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
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.appConfigs.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
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
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	var req domain.AppConfigInput
	if !httpx.BindJSON(c, &req) {
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
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInvalidArgument(c, err.Error())
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
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.appConfigs.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

// ListAppConfigs godoc
// @Summary List app configs
// @Tags AppConfig
// @Produce json
// @Param application_id query string true "Application ID"
// @Param environment_id query string true "Environment ID"
// @Param name query string false "Name"
// @Param page query int false "Page"
// @Param page_size query int false "Page size"
// @Success 200 {object} httpx.ListResponse[domain.AppConfig]
// @Router /api/v1/app-configs [get]
func (h *Handler) ListAppConfigs(c *gin.Context) {
	var filter appconfig.AppConfigListFilter
	appID, ok := httpx.ParseUUIDQuery(c, "application_id")
	if !ok || appID == nil {
		if ok {
			httpx.WriteInvalidArgument(c, "application_id is required")
		}
		return
	}
	environmentID := c.Query("environment_id")
	if environmentID == "" {
		httpx.WriteInvalidArgument(c, "environment_id is required")
		return
	}
	filter.ApplicationID = appID
	filter.EnvironmentID = environmentID
	filter.Name = c.Query("name")
	filter.IncludeDeleted = httpx.IncludeDeleted(c)
	items, err := h.appConfigs.List(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, items)
}

// SyncAppConfig godoc
// @Summary Sync app config from fixed config repo
// @Tags AppConfig
// @Produce json
// @Param id path string true "AppConfig ID"
// @Success 200 {object} httpx.DataResponse[domain.AppConfigRevision]
// @Router /api/v1/app-configs/{id}/sync-from-repo [post]
func (h *Handler) SyncAppConfig(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	result, err := h.appConfigs.Sync(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			httpx.WriteNotFound(c, "not found")
		case errors.Is(err, appconfig.ErrConfigSourceNotFound), errors.Is(err, appconfig.ErrConfigRepositoryUnavailable), errors.Is(err, appconfig.ErrConfigRepositorySyncFailed):
			httpx.WriteFailedPrecondition(c, http.StatusFailedDependency, err.Error())
		default:
			httpx.WriteInternalError(c, err)
		}
		return
	}
	httpx.WriteData(c, http.StatusOK, result.Revision)
}
