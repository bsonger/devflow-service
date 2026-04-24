package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	clustersvc "github.com/bsonger/devflow-service/internal/cluster/application"
	clusterdomain "github.com/bsonger/devflow-service/internal/cluster/domain"
	platformhttpx "github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type clusterService interface {
	Create(context.Context, *clusterdomain.Cluster) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*clusterdomain.Cluster, error)
	Update(context.Context, *clusterdomain.Cluster) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, clustersvc.ListFilter) ([]clusterdomain.Cluster, error)
}

type Handler struct {
	svc clusterService
}

type CreateClusterRequest struct {
	Name              string                    `json:"name" binding:"required"`
	Server            string                    `json:"server" binding:"required"`
	KubeConfig        string                    `json:"kubeconfig" binding:"required"`
	ArgoCDClusterName string                    `json:"argocd_cluster_name,omitempty"`
	Description       string                    `json:"description,omitempty"`
	Labels            []clusterdomain.LabelItem `json:"labels,omitempty"`
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

func (h *Handler) Create(c *gin.Context) {
	var req CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	cluster := &clusterdomain.Cluster{
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

	platformhttpx.WriteData(c, http.StatusCreated, cluster)
}

func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	cluster, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		writeClusterError(c, err)
		return
	}

	platformhttpx.WriteData(c, http.StatusOK, cluster)
}

func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	var req UpdateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	cluster := clusterdomain.Cluster{
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

	platformhttpx.WriteNoContent(c)
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeClusterError(c, err)
		return
	}

	platformhttpx.WriteNoContent(c)
}

func (h *Handler) List(c *gin.Context) {
	filter := clustersvc.ListFilter{
		IncludeDeleted: platformhttpx.IncludeDeleted(c),
		Name:           c.Query("name"),
	}

	clusters, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		writeClusterError(c, err)
		return
	}

	paging, err := platformhttpx.ParsePagination(c)
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	total := len(clusters)
	clusters = platformhttpx.PaginateSlice(clusters, paging)
	platformhttpx.WriteList(c, http.StatusOK, clusters, paging, total)
}

func writeClusterError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		platformhttpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
	case errors.Is(err, clustersvc.ErrClusterConflict):
		platformhttpx.WriteError(c, http.StatusConflict, "conflict", err.Error(), nil)
	case errors.Is(err, clustersvc.ErrClusterNameRequired),
		errors.Is(err, clustersvc.ErrClusterServerRequired),
		errors.Is(err, clustersvc.ErrClusterKubeConfigRequired),
		errors.Is(err, clustersvc.ErrClusterOnboardingMalformed):
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
	case errors.Is(err, clustersvc.ErrClusterOnboardingTimeout):
		platformhttpx.WriteError(c, http.StatusGatewayTimeout, "deadline_exceeded", err.Error(), nil)
	case errors.Is(err, clustersvc.ErrClusterOnboardingFailed):
		platformhttpx.WriteError(c, http.StatusConflict, "failed_precondition", err.Error(), nil)
	default:
		platformhttpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
	}
}
