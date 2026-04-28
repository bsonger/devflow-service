package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/httpx"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	runtimeservice "github.com/bsonger/devflow-service/internal/runtime/service"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type runtimeService interface {
	CreateRuntimeSpec(context.Context, runtimeservice.CreateRuntimeSpecInput) (*runtimedomain.RuntimeSpec, error)
	ListRuntimeSpecs(context.Context) ([]*runtimedomain.RuntimeSpec, error)
	GetRuntimeSpec(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpec, error)
	DeleteRuntimeSpecByApplicationEnv(context.Context, uuid.UUID, string) error
	CreateRuntimeSpecRevision(context.Context, uuid.UUID, runtimeservice.CreateRuntimeSpecRevisionInput) (*runtimedomain.RuntimeSpecRevision, error)
	ListRuntimeSpecRevisions(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeSpecRevision, error)
	GetRuntimeSpecRevision(context.Context, uuid.UUID) (*runtimedomain.RuntimeSpecRevision, error)
	ListObservedPods(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error)
	SyncObservedPod(context.Context, runtimeservice.SyncObservedPodInput) (*runtimedomain.RuntimeObservedPod, error)
	DeleteObservedPod(context.Context, runtimeservice.DeleteObservedPodInput) error
	DeletePod(context.Context, uuid.UUID, string, string) error
	RestartDeployment(context.Context, uuid.UUID, string, string) error
	ListRuntimeOperations(context.Context, uuid.UUID) ([]*runtimedomain.RuntimeOperation, error)
}

type Handler struct {
	runtime runtimeService
}

type CreateRuntimeSpecRequest struct {
	ApplicationID uuid.UUID `json:"application_id"`
	Environment   string    `json:"environment"`
}

type DeleteRuntimeSpecRequest struct {
	ApplicationID uuid.UUID `json:"application_id"`
	Environment   string    `json:"environment"`
}

type CreateRuntimeSpecRevisionRequest struct {
	Replicas         int    `json:"replicas"`
	HealthThresholds string `json:"health_thresholds"`
	Resources        string `json:"resources"`
	Autoscaling      string `json:"autoscaling"`
	Scheduling       string `json:"scheduling"`
	PodEnvs          string `json:"pod_envs"`
	CreatedBy        string `json:"created_by"`
}

type SyncObservedPodRequest struct {
	ApplicationID uuid.UUID                     `json:"application_id"`
	Environment   string                        `json:"environment"`
	Namespace     string                        `json:"namespace"`
	PodName       string                        `json:"pod_name"`
	Phase         string                        `json:"phase"`
	Ready         bool                          `json:"ready"`
	Restarts      int                           `json:"restarts"`
	NodeName      string                        `json:"node_name,omitempty"`
	PodIP         string                        `json:"pod_ip,omitempty"`
	HostIP        string                        `json:"host_ip,omitempty"`
	OwnerKind     string                        `json:"owner_kind,omitempty"`
	OwnerName     string                        `json:"owner_name,omitempty"`
	Labels        map[string]string             `json:"labels,omitempty"`
	Containers    []ObservedPodContainerRequest `json:"containers,omitempty"`
	ObservedAt    time.Time                     `json:"observed_at"`
}

type DeleteObservedPodRequest struct {
	ApplicationID uuid.UUID `json:"application_id"`
	Environment   string    `json:"environment"`
	Namespace     string    `json:"namespace"`
	PodName       string    `json:"pod_name"`
	ObservedAt    time.Time `json:"observed_at"`
}

type ObservedPodContainerRequest struct {
	Name         string `json:"name"`
	Image        string `json:"image,omitempty"`
	ImageID      string `json:"image_id,omitempty"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restart_count"`
	State        string `json:"state,omitempty"`
}

func NewHandler(runtime runtimeService) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	runtimeSpecs := rg.Group("/runtime-specs")
	{
		runtimeSpecs.POST("", h.CreateRuntimeSpec)
		runtimeSpecs.GET("", h.ListRuntimeSpecs)
		runtimeSpecs.DELETE("", h.DeleteRuntimeSpec)
		runtimeSpecs.GET("/:id", h.GetRuntimeSpec)
		runtimeSpecs.POST("/:id/revisions", h.CreateRuntimeSpecRevision)
		runtimeSpecs.GET("/:id/revisions", h.ListRuntimeSpecRevisions)
		runtimeSpecs.GET("/:id/pods", h.ListObservedPods)
		runtimeSpecs.POST("/:id/pods/:pod_name/delete", h.DeletePod)
		runtimeSpecs.POST("/:id/deployments/:deployment_name/restart", h.RestartDeployment)
		runtimeSpecs.GET("/:id/operations", h.ListRuntimeOperations)
	}

	rg.GET("/runtime-spec-revisions/:id", h.GetRuntimeSpecRevision)
}

func (h *Handler) RegisterInternalRoutes(rg *gin.RouterGroup) {
	observer := rg.Group("/internal/runtime-spec-pods")
	{
		observer.POST("/sync", h.SyncObservedPod)
		observer.POST("/delete", h.DeleteObservedPod)
	}
}

func (h *Handler) CreateRuntimeSpec(c *gin.Context) {
	var req CreateRuntimeSpecRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	item, err := h.runtime.CreateRuntimeSpec(c.Request.Context(), runtimeservice.CreateRuntimeSpecInput{
		ApplicationID: req.ApplicationID,
		Environment:   req.Environment,
	})
	if err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WriteData(c, http.StatusCreated, item)
}

func (h *Handler) ListRuntimeSpecs(c *gin.Context) {
	items, err := h.runtime.ListRuntimeSpecs(c.Request.Context())
	if err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, items)
}

func (h *Handler) GetRuntimeSpec(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	item, err := h.runtime.GetRuntimeSpec(c.Request.Context(), id)
	if err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WriteData(c, http.StatusOK, item)
}

func (h *Handler) DeleteRuntimeSpec(c *gin.Context) {
	var req DeleteRuntimeSpecRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := h.runtime.DeleteRuntimeSpecByApplicationEnv(c.Request.Context(), req.ApplicationID, req.Environment); err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func (h *Handler) CreateRuntimeSpecRevision(c *gin.Context) {
	runtimeSpecID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	var req CreateRuntimeSpecRevisionRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	item, err := h.runtime.CreateRuntimeSpecRevision(c.Request.Context(), runtimeSpecID, runtimeservice.CreateRuntimeSpecRevisionInput{
		Replicas:         req.Replicas,
		HealthThresholds: req.HealthThresholds,
		Resources:        req.Resources,
		Autoscaling:      req.Autoscaling,
		Scheduling:       req.Scheduling,
		PodEnvs:          req.PodEnvs,
		CreatedBy:        req.CreatedBy,
	})
	if err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WriteData(c, http.StatusCreated, item)
}

func (h *Handler) ListRuntimeSpecRevisions(c *gin.Context) {
	runtimeSpecID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	items, err := h.runtime.ListRuntimeSpecRevisions(c.Request.Context(), runtimeSpecID)
	if err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, items)
}

func (h *Handler) GetRuntimeSpecRevision(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.runtime.GetRuntimeSpecRevision(c.Request.Context(), id)
	if err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WriteData(c, http.StatusOK, item)
}

func (h *Handler) ListObservedPods(c *gin.Context) {
	runtimeSpecID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	items, err := h.runtime.ListObservedPods(c.Request.Context(), runtimeSpecID)
	if err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, items)
}

func (h *Handler) DeletePod(c *gin.Context) {
	runtimeSpecID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	podName := strings.TrimSpace(c.Param("pod_name"))
	if podName == "" {
		httpx.WriteInvalidArgument(c, "pod_name is required")
		return
	}
	var req struct {
		Operator string `json:"operator"`
	}
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := h.runtime.DeletePod(c.Request.Context(), runtimeSpecID, podName, req.Operator); err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func (h *Handler) RestartDeployment(c *gin.Context) {
	runtimeSpecID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	deploymentName := strings.TrimSpace(c.Param("deployment_name"))
	if deploymentName == "" {
		httpx.WriteInvalidArgument(c, "deployment_name is required")
		return
	}
	var req struct {
		Operator string `json:"operator"`
	}
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := h.runtime.RestartDeployment(c.Request.Context(), runtimeSpecID, deploymentName, req.Operator); err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func (h *Handler) ListRuntimeOperations(c *gin.Context) {
	runtimeSpecID, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	items, err := h.runtime.ListRuntimeOperations(c.Request.Context(), runtimeSpecID)
	if err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, items)
}

func (h *Handler) SyncObservedPod(c *gin.Context) {
	var req SyncObservedPodRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	_, err := h.runtime.SyncObservedPod(c.Request.Context(), runtimeservice.SyncObservedPodInput{
		ApplicationID: req.ApplicationID,
		Environment:   req.Environment,
		Namespace:     req.Namespace,
		PodName:       req.PodName,
		Phase:         req.Phase,
		Ready:         req.Ready,
		Restarts:      req.Restarts,
		NodeName:      req.NodeName,
		PodIP:         req.PodIP,
		HostIP:        req.HostIP,
		OwnerKind:     req.OwnerKind,
		OwnerName:     req.OwnerName,
		Labels:        req.Labels,
		Containers:    mapObservedPodContainerRequests(req.Containers),
		ObservedAt:    req.ObservedAt,
	})
	if err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func (h *Handler) DeleteObservedPod(c *gin.Context) {
	var req DeleteObservedPodRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := h.runtime.DeleteObservedPod(c.Request.Context(), runtimeservice.DeleteObservedPodInput{
		ApplicationID: req.ApplicationID,
		Environment:   req.Environment,
		Namespace:     req.Namespace,
		PodName:       req.PodName,
		ObservedAt:    req.ObservedAt,
	}); err != nil {
		writeRuntimeError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func mapObservedPodContainerRequests(in []ObservedPodContainerRequest) []runtimeservice.ObservedPodContainerInput {
	if len(in) == 0 {
		return nil
	}
	out := make([]runtimeservice.ObservedPodContainerInput, 0, len(in))
	for _, item := range in {
		out = append(out, runtimeservice.ObservedPodContainerInput{
			Name:         item.Name,
			Image:        item.Image,
			ImageID:      item.ImageID,
			Ready:        item.Ready,
			RestartCount: item.RestartCount,
			State:        item.State,
		})
	}
	return out
}

func writeRuntimeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteNotFound(c, "not found")
	case sharederrs.HasCode(err, sharederrs.CodeInvalidArgument):
		httpx.WriteInvalidArgument(c, err.Error())
	case sharederrs.HasCode(err, sharederrs.CodeConflict):
		httpx.WriteConflict(c, err.Error())
	case sharederrs.HasCode(err, sharederrs.CodeNotFound):
		httpx.WriteNotFound(c, err.Error())
	case sharederrs.HasCode(err, sharederrs.CodeFailedPrecondition):
		httpx.WriteFailedPrecondition(c, http.StatusPreconditionFailed, err.Error())
	default:
		httpx.WriteInternalError(c, err)
	}
}
