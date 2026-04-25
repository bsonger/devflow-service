package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bsonger/devflow-service/internal/cluster/domain"
	"github.com/bsonger/devflow-service/internal/cluster/service"
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type clusterService interface {
	Create(context.Context, *domain.Cluster) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*domain.Cluster, error)
	Update(context.Context, *domain.Cluster) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, service.ListFilter) ([]domain.Cluster, error)
}

type Handler struct {
	svc clusterService
}

type CreateClusterRequest struct {
	Name              string             `json:"name" binding:"required"`
	Server            string             `json:"server" binding:"required"`
	KubeConfig        string             `json:"kubeconfig" binding:"required"`
	ArgoCDClusterName string             `json:"argocd_cluster_name,omitempty"`
	Description       string             `json:"description,omitempty"`
	Labels            []domain.LabelItem `json:"labels,omitempty"`
}

type UpdateClusterRequest = CreateClusterRequest

func NewHandler(svc clusterService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	cluster := rg.Group("/clusters")
	cluster.GET("", h.List)
	cluster.GET("/:id", h.Get)
	cluster.POST("", h.Create)
	cluster.PUT("/:id", h.Update)
	cluster.DELETE("/:id", h.Delete)
}

// Create godoc
// @Summary 创建集群
// @Description 创建一个新的集群
// @Tags Cluster
// @Accept json
// @Produce json
// @Param data body CreateClusterRequest true "Cluster Data"
// @Success 201 {object} httpx.DataResponse[domain.Cluster]
// @Router /api/v1/clusters [post]
func (h *Handler) Create(c *gin.Context) {
	var req CreateClusterRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	cluster := &domain.Cluster{
		Name:              req.Name,
		Server:            req.Server,
		KubeConfig:        req.KubeConfig,
		ArgoCDClusterName: req.ArgoCDClusterName,
		Description:       req.Description,
		Labels:            req.Labels,
	}
	cluster.WithCreateDefault()

	_, err := h.svc.Create(c.Request.Context(), cluster)
	if err != nil {
		writeClusterError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusCreated, cluster)
}

// Get godoc
// @Summary 获取集群
// @Tags Cluster
// @Param id path string true "Cluster ID"
// @Success 200 {object} httpx.DataResponse[domain.Cluster]
// @Router /api/v1/clusters/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	cluster, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		writeClusterError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusOK, cluster)
}

// Update godoc
// @Summary 更新集群
// @Tags Cluster
// @Param id path string true "Cluster ID"
// @Param data body UpdateClusterRequest true "Cluster Data"
// @Success 204
// @Router /api/v1/clusters/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	var req UpdateClusterRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	cluster := domain.Cluster{
		Name:              req.Name,
		Server:            req.Server,
		KubeConfig:        req.KubeConfig,
		ArgoCDClusterName: req.ArgoCDClusterName,
		Description:       req.Description,
		Labels:            req.Labels,
	}
	cluster.SetID(id)

	if err := h.svc.Update(c.Request.Context(), &cluster); err != nil {
		writeClusterError(c, err)
		return
	}

	httpx.WriteNoContent(c)
}

// Delete godoc
// @Summary 删除集群
// @Tags Cluster
// @Param id path string true "Cluster ID"
// @Success 204
// @Router /api/v1/clusters/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeClusterError(c, err)
		return
	}

	httpx.WriteNoContent(c)
}

// List godoc
// @Summary 获取集群列表
// @Tags Cluster
// @Success 200 {object} httpx.ListResponse[domain.Cluster]
// @Router /api/v1/clusters [get]
func (h *Handler) List(c *gin.Context) {
	filter := service.ListFilter{
		IncludeDeleted: httpx.IncludeDeleted(c),
		Name:           c.Query("name"),
	}

	clusters, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		writeClusterError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, clusters)
}

func writeClusterError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteNotFound(c, "not found")
	case errors.Is(err, service.ErrClusterConflict):
		httpx.WriteConflict(c, err.Error())
	case errors.Is(err, service.ErrClusterNameRequired),
		errors.Is(err, service.ErrClusterServerRequired),
		errors.Is(err, service.ErrClusterKubeConfigRequired),
		errors.Is(err, service.ErrClusterOnboardingMalformed):
		httpx.WriteInvalidArgument(c, err.Error())
	case errors.Is(err, service.ErrClusterOnboardingTimeout):
		httpx.WriteError(c, http.StatusGatewayTimeout, "deadline_exceeded", err.Error(), nil)
	case errors.Is(err, service.ErrClusterOnboardingFailed):
		httpx.WriteFailedPrecondition(c, http.StatusConflict, err.Error())
	default:
		httpx.WriteInternalError(c, err)
	}
}
