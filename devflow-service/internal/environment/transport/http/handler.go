package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	envsvc "github.com/bsonger/devflow-service/internal/environment/application"
	envdomain "github.com/bsonger/devflow-service/internal/environment/domain"
	platformhttpx "github.com/bsonger/devflow-service/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type environmentService interface {
	Create(context.Context, *envdomain.Environment) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (*envdomain.Environment, error)
	Update(context.Context, *envdomain.Environment) error
	Delete(context.Context, uuid.UUID) error
	List(context.Context, envsvc.ListFilter) ([]envdomain.Environment, error)
}

type Handler struct {
	svc environmentService
}

type CreateEnvironmentRequest struct {
	Name        string                `json:"name" binding:"required"`
	ClusterID   uuid.UUID             `json:"cluster_id" binding:"required"`
	Description string                `json:"description,omitempty"`
	Labels      []envdomain.LabelItem `json:"labels,omitempty"`
}

type UpdateEnvironmentRequest = CreateEnvironmentRequest

func NewHandler(svc environmentService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	environment := rg.Group("/environments")
	environment.GET("", h.List)
	environment.GET("/:id", h.Get)
	environment.POST("", h.Create)
	environment.PUT("/:id", h.Update)
	environment.DELETE("/:id", h.Delete)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	environment := &envdomain.Environment{
		Name:        req.Name,
		ClusterID:   req.ClusterID,
		Description: req.Description,
		Labels:      req.Labels,
	}
	environment.WithCreateDefault()

	_, err := h.svc.Create(c.Request.Context(), environment)
	if err != nil {
		writeEnvironmentError(c, err)
		return
	}

	platformhttpx.WriteData(c, http.StatusCreated, environment)
}

func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	environment, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		writeEnvironmentError(c, err)
		return
	}

	platformhttpx.WriteData(c, http.StatusOK, environment)
}

func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid id", nil)
		return
	}

	var req UpdateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	environment := envdomain.Environment{
		Name:        req.Name,
		ClusterID:   req.ClusterID,
		Description: req.Description,
		Labels:      req.Labels,
	}
	environment.SetID(id)

	if err := h.svc.Update(c.Request.Context(), &environment); err != nil {
		writeEnvironmentError(c, err)
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
		writeEnvironmentError(c, err)
		return
	}

	platformhttpx.WriteNoContent(c)
}

func (h *Handler) List(c *gin.Context) {
	filter := envsvc.ListFilter{
		IncludeDeleted: platformhttpx.IncludeDeleted(c),
		Name:           c.Query("name"),
	}
	if clusterID := c.Query("cluster_id"); clusterID != "" {
		id, err := uuid.Parse(clusterID)
		if err != nil {
			platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", "invalid cluster_id", nil)
			return
		}
		filter.ClusterID = &id
	}

	environments, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		writeEnvironmentError(c, err)
		return
	}

	paging, err := platformhttpx.ParsePagination(c)
	if err != nil {
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	total := len(environments)
	environments = platformhttpx.PaginateSlice(environments, paging)
	platformhttpx.WriteList(c, http.StatusOK, environments, paging, total)
}

func writeEnvironmentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		platformhttpx.WriteError(c, http.StatusNotFound, "not_found", "not found", nil)
	case errors.Is(err, envsvc.ErrEnvironmentConflict):
		platformhttpx.WriteError(c, http.StatusConflict, "conflict", err.Error(), nil)
	case errors.Is(err, envsvc.ErrEnvironmentNameRequired),
		errors.Is(err, envsvc.ErrEnvironmentClusterRequired),
		errors.Is(err, envsvc.ErrClusterReferenceNotFound):
		platformhttpx.WriteError(c, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
	default:
		platformhttpx.WriteError(c, http.StatusInternalServerError, "internal", err.Error(), nil)
	}
}
