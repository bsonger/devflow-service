package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/config/domain"
	"github.com/bsonger/devflow-service/internal/config/service"
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type appConfigService interface {
	Create(ctx context.Context, cfg *domain.AppConfig) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.AppConfig, error)
	Update(ctx context.Context, cfg *domain.AppConfig) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter service.AppConfigListFilter) ([]domain.AppConfig, error)
	Sync(ctx context.Context, id uuid.UUID) (*service.AppConfigSyncResult, error)
}

type workloadConfigService interface {
	Create(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.WorkloadConfig, error)
	Update(ctx context.Context, item *domain.WorkloadConfig) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter service.WorkloadConfigListFilter) ([]domain.WorkloadConfig, error)
}

type Handler struct {
	appConfigs     appConfigService
	workloadConfigs workloadConfigService
}

func NewHandler(appConfigs appConfigService, workloadConfigs workloadConfigService) *Handler {
	return &Handler{appConfigs: appConfigs, workloadConfigs: workloadConfigs}
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
	workloads := rg.Group("/workload-configs")
	{
		workloads.GET("", h.ListWorkloadConfigs)
		workloads.GET("/:id", h.GetWorkloadConfig)
		workloads.POST("", h.CreateWorkloadConfig)
		workloads.PUT("/:id", h.UpdateWorkloadConfig)
		workloads.DELETE("/:id", h.DeleteWorkloadConfig)
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
	var filter service.AppConfigListFilter
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
		case errors.Is(err, service.ErrConfigSourceNotFound), errors.Is(err, service.ErrConfigRepositoryUnavailable), errors.Is(err, service.ErrConfigRepositorySyncFailed):
			httpx.WriteError(c, http.StatusFailedDependency, "failed_precondition", err.Error(), nil)
		default:
			httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		}
		return
	}
	httpx.WriteData(c, http.StatusOK, result.Revision)
}

// WorkloadConfig handlers

// CreateWorkloadConfig godoc
// @Summary Create workload config
// @Tags WorkloadConfig
// @Accept json
// @Produce json
// @Param data body domain.WorkloadConfigInput true "WorkloadConfig data"
// @Success 201 {object} httpx.DataResponse[domain.WorkloadConfig]
// @Router /api/v1/workload-configs [post]
func (h *Handler) CreateWorkloadConfig(c *gin.Context) {
	var req domain.WorkloadConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	item := &domain.WorkloadConfig{
		ApplicationID: req.ApplicationID,
		EnvironmentID: req.EnvironmentID,
		Name:          req.Name,
		Description:   req.Description,
		Replicas:      req.Replicas,
		Exposed:       req.Exposed,
		Resources:     req.Resources,
		Probes:        req.Probes,
		Env:           req.Env,
		Labels:        req.Labels,
		WorkloadType:  req.WorkloadType,
		Strategy:      req.Strategy,
	}
	item.WithCreateDefault()
	if _, err := h.workloadConfigs.Create(c.Request.Context(), item); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	httpx.WriteData(c, http.StatusCreated, item)
}

// GetWorkloadConfig godoc
// @Summary Get workload config
// @Tags WorkloadConfig
// @Produce json
// @Param id path string true "WorkloadConfig ID"
// @Success 200 {object} httpx.DataResponse[domain.WorkloadConfig]
// @Router /api/v1/workload-configs/{id} [get]
func (h *Handler) GetWorkloadConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}
	item, err := h.workloadConfigs.Get(c.Request.Context(), id)
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

// UpdateWorkloadConfig godoc
// @Summary Update workload config
// @Tags WorkloadConfig
// @Accept json
// @Param id path string true "WorkloadConfig ID"
// @Param data body domain.WorkloadConfigInput true "WorkloadConfig data"
// @Success 204
// @Router /api/v1/workload-configs/{id} [put]
func (h *Handler) UpdateWorkloadConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}
	var req domain.WorkloadConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	item := &domain.WorkloadConfig{
		ApplicationID: req.ApplicationID,
		EnvironmentID: req.EnvironmentID,
		Name:          req.Name,
		Description:   req.Description,
		Replicas:      req.Replicas,
		Exposed:       req.Exposed,
		Resources:     req.Resources,
		Probes:        req.Probes,
		Env:           req.Env,
		Labels:        req.Labels,
		WorkloadType:  req.WorkloadType,
		Strategy:      req.Strategy,
	}
	item.SetID(id)
	if err := h.workloadConfigs.Update(c.Request.Context(), item); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	httpx.WriteNoContent(c)
}

// DeleteWorkloadConfig godoc
// @Summary Delete workload config
// @Tags WorkloadConfig
// @Param id path string true "WorkloadConfig ID"
// @Success 204
// @Router /api/v1/workload-configs/{id} [delete]
func (h *Handler) DeleteWorkloadConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}
	if err := h.workloadConfigs.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	httpx.WriteNoContent(c)
}

// ListWorkloadConfigs godoc
// @Summary List workload configs
// @Tags WorkloadConfig
// @Produce json
// @Param application_id query string false "Application ID"
// @Param environment_id query string false "Environment ID"
// @Param name query string false "Name"
// @Param page query int false "Page"
// @Param page_size query int false "Page size"
// @Success 200 {object} httpx.ListResponse[domain.WorkloadConfig]
// @Router /api/v1/workload-configs [get]
func (h *Handler) ListWorkloadConfigs(c *gin.Context) {
	var filter service.WorkloadConfigListFilter
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
	items, err := h.workloadConfigs.List(c.Request.Context(), filter)
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
