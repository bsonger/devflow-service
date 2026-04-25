package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	workloadconfig "github.com/bsonger/devflow-service/internal/workloadconfig/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type workloadConfigService interface {
	Create(ctx context.Context, item *domain.WorkloadConfig) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.WorkloadConfig, error)
	Update(ctx context.Context, item *domain.WorkloadConfig) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter workloadconfig.WorkloadConfigListFilter) ([]domain.WorkloadConfig, error)
}

type Handler struct {
	workloadConfigs workloadConfigService
}

func NewHandler(workloadConfigs workloadConfigService) *Handler {
	return &Handler{workloadConfigs: workloadConfigs}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	workloads := rg.Group("/workload-configs")
	{
		workloads.GET("", h.ListWorkloadConfigs)
		workloads.GET("/:id", h.GetWorkloadConfig)
		workloads.POST("", h.CreateWorkloadConfig)
		workloads.PUT("/:id", h.UpdateWorkloadConfig)
		workloads.DELETE("/:id", h.DeleteWorkloadConfig)
	}
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
	if !httpx.BindJSON(c, &req) {
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
		httpx.WriteInvalidArgument(c, err.Error())
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
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.workloadConfigs.Get(c.Request.Context(), id)
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

// UpdateWorkloadConfig godoc
// @Summary Update workload config
// @Tags WorkloadConfig
// @Accept json
// @Param id path string true "WorkloadConfig ID"
// @Param data body domain.WorkloadConfigInput true "WorkloadConfig data"
// @Success 204
// @Router /api/v1/workload-configs/{id} [put]
func (h *Handler) UpdateWorkloadConfig(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	var req domain.WorkloadConfigInput
	if !httpx.BindJSON(c, &req) {
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
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInvalidArgument(c, err.Error())
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
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.workloadConfigs.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
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
	var filter workloadconfig.WorkloadConfigListFilter
	appID, ok := httpx.ParseUUIDQuery(c, "application_id")
	if !ok {
		return
	}
	if appID != nil {
		filter.ApplicationID = appID
	}
	filter.EnvironmentID = c.Query("environment_id")
	filter.Name = c.Query("name")
	filter.IncludeDeleted = httpx.IncludeDeleted(c)
	items, err := h.workloadConfigs.List(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, items)
}
