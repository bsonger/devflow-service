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

var ClusterRouteApi = NewClusterHandler()

type clusterService interface {
	Create(ctx context.Context, cluster *domain.Cluster) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Cluster, error)
	Update(ctx context.Context, cluster *domain.Cluster) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter app.ClusterListFilter) ([]domain.Cluster, error)
}

type ClusterHandler struct {
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

func NewClusterHandler() *ClusterHandler {
	return &ClusterHandler{svc: app.ClusterService}
}

// Create
// @Summary 创建集群
// @Description 创建一个新的集群
// @Tags Cluster
// @Accept json
// @Produce json
// @Param data body api.CreateClusterRequest true "Cluster Data"
// @Success 201 {object} httpx.DataResponse[domain.Cluster]
// @Router /api/v1/clusters [post]
func (h *ClusterHandler) Create(c *gin.Context) {
	var req CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
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

// Get
// @Summary 获取集群
// @Tags Cluster
// @Param id path string true "Cluster ID"
// @Success 200 {object} httpx.DataResponse[domain.Cluster]
// @Router /api/v1/clusters/{id} [get]
func (h *ClusterHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	cluster, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		writeClusterError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusOK, cluster)
}

// Update
// @Summary 更新集群
// @Tags Cluster
// @Param id path string true "Cluster ID"
// @Param data body api.UpdateClusterRequest true "Cluster Data"
// @Success 204
// @Router /api/v1/clusters/{id} [put]
func (h *ClusterHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	var req UpdateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
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

// Delete
// @Summary 删除集群
// @Tags Cluster
// @Param id path string true "Cluster ID"
// @Success 204
// @Router /api/v1/clusters/{id} [delete]
func (h *ClusterHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeClusterError(c, err)
		return
	}

	httpx.WriteNoContent(c)
}

// List
// @Summary 获取集群列表
// @Tags Cluster
// @Success 200 {object} httpx.ListResponse[domain.Cluster]
// @Router /api/v1/clusters [get]
func (h *ClusterHandler) List(c *gin.Context) {
	filter := app.ClusterListFilter{
		IncludeDeleted: httpx.IncludeDeleted(c),
		Name:           c.Query("name"),
	}

	clusters, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		writeClusterError(c, err)
		return
	}

	paging, err := httpx.ParsePagination(c)
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	total := len(clusters)
	clusters = httpx.PaginateSlice(clusters, paging)
	httpx.WriteList(c, http.StatusOK, clusters, paging, total)
}

func writeClusterError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
	case errors.Is(err, app.ErrClusterConflict):
		httpx.WriteError(c, http.StatusConflict, "conflict", err.Error(), nil)
	case errors.Is(err, app.ErrClusterNameRequired),
		errors.Is(err, app.ErrClusterServerRequired),
		errors.Is(err, app.ErrClusterKubeConfigRequired),
		errors.Is(err, app.ErrClusterOnboardingMalformed):
		httpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
	case errors.Is(err, app.ErrClusterOnboardingTimeout):
		httpx.WriteError(c, http.StatusGatewayTimeout, "deadline_exceeded", err.Error(), nil)
	case errors.Is(err, app.ErrClusterOnboardingFailed):
		httpx.WriteError(c, http.StatusConflict, "failed_precondition", err.Error(), nil)
	default:
		httpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
	}
}
