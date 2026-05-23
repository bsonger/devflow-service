package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	manifestservice "github.com/bsonger/devflow-service/internal/manifest/service"
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type manifestService interface {
	CreateManifest(context.Context, *manifestdomain.CreateManifestRequest) (*manifestdomain.Manifest, error)
	List(context.Context, manifestdomain.ManifestListFilter) ([]manifestdomain.Manifest, error)
	Get(context.Context, uuid.UUID) (*manifestdomain.Manifest, error)
	GetResources(context.Context, uuid.UUID) (*manifestdomain.ManifestResourcesView, error)
	Delete(context.Context, uuid.UUID) error
}

type applicationReader interface {
	Get(context.Context, uuid.UUID) (*releasesupport.ApplicationProjection, error)
}

type ManifestHandler struct {
	svc  manifestService
	apps applicationReader
}

type ManifestResponse struct {
	Data *ManifestDoc `json:"data"`
}

type ManifestListResponse struct {
	Data       []ManifestDoc    `json:"data"`
	Pagination httpx.Pagination `json:"pagination"`
}

type ManifestResourcesResponse struct {
	Data *ManifestResourcesViewDoc `json:"data"`
}

func NewManifestHandler(svc manifestService) *ManifestHandler {
	return &ManifestHandler{svc: svc, apps: releasesupport.ApplicationService}
}

func (h *ManifestHandler) RegisterRoutes(rg *gin.RouterGroup) {
	manifests := rg.Group("/manifests")
	manifests.POST("", h.Create)
	manifests.GET("", h.List)
	manifests.GET("/:id", h.Get)
	manifests.GET("/:id/resources", h.GetResources)
	manifests.DELETE("/:id", h.Delete)
}

// CreateManifest godoc
// @Summary Create manifest
// @Tags Manifest
// @Accept json
// @Produce json
// @Param data body CreateManifestRequestDoc true "Manifest create request"
// @Success 201 {object} ManifestResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/manifests [post]
func (h *ManifestHandler) Create(c *gin.Context) {
	var req manifestdomain.CreateManifestRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	item, err := h.svc.CreateManifest(c.Request.Context(), &req)
	if err != nil {
		writeManifestError(c, err)
		return
	}
	httpx.WriteData(c, http.StatusCreated, h.enrichManifestDoc(c.Request.Context(), *item))
}

// ListManifests godoc
// @Summary List manifests
// @Tags Manifest
// @Produce json
// @Param application_id query string false "Application ID"
// @Param page query int false "Page"
// @Param page_size query int false "Page size"
// @Success 200 {object} ManifestListResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/manifests [get]
func (h *ManifestHandler) List(c *gin.Context) {
	filter := manifestdomain.ManifestListFilter{IncludeDeleted: httpx.IncludeDeleted(c)}
	applicationId, ok := httpx.ParseUUIDQuery(c, "application_id")
	if !ok {
		return
	}
	if applicationId != nil {
		filter.ApplicationID = applicationId
	}
	items, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, h.enrichManifestDocs(c.Request.Context(), items))
}

// GetManifest godoc
// @Summary Get manifest
// @Tags Manifest
// @Produce json
// @Param id path string true "Manifest ID"
// @Success 200 {object} ManifestResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/manifests/{id} [get]
func (h *ManifestHandler) Get(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WriteData(c, http.StatusOK, h.enrichManifestDoc(c.Request.Context(), *item))
}

// GetManifestResources godoc
// @Summary Get manifest frozen resources
// @Tags Manifest
// @Produce json
// @Param id path string true "Manifest ID"
// @Success 200 {object} ManifestResourcesResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/manifests/{id}/resources [get]
func (h *ManifestHandler) GetResources(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetResources(c.Request.Context(), id)
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

// DeleteManifest godoc
// @Summary Delete manifest
// @Tags Manifest
// @Param id path string true "Manifest ID"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /api/v1/manifests/{id} [delete]
func (h *ManifestHandler) Delete(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WriteNoContent(c)
}

func writeManifestError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteNotFound(c, "not found")
	case errors.Is(err, manifestservice.ErrManifestWorkloadConfigMissing),
		errors.Is(err, manifestservice.ErrManifestRepositoryMissing),
		errors.Is(err, manifestservice.ErrManifestImageNotDeployable):
		httpx.WriteFailedPrecondition(c, http.StatusConflict, err.Error())
	default:
		httpx.WriteInternalError(c, err)
	}
}

func (h *ManifestHandler) enrichManifestDocs(ctx context.Context, manifests []manifestdomain.Manifest) []ManifestDoc {
	if len(manifests) == 0 {
		return []ManifestDoc{}
	}

	names := h.lookupApplicationNames(ctx, manifests)
	items := make([]ManifestDoc, 0, len(manifests))
	for _, manifest := range manifests {
		items = append(items, buildManifestDoc(manifest, names[manifest.ApplicationID]))
	}
	return items
}

func (h *ManifestHandler) enrichManifestDoc(ctx context.Context, manifest manifestdomain.Manifest) ManifestDoc {
	names := h.lookupApplicationNames(ctx, []manifestdomain.Manifest{manifest})
	return buildManifestDoc(manifest, names[manifest.ApplicationID])
}

func (h *ManifestHandler) lookupApplicationNames(ctx context.Context, manifests []manifestdomain.Manifest) map[uuid.UUID]string {
	names := make(map[uuid.UUID]string, len(manifests))
	if h.apps == nil {
		return names
	}

	seen := make(map[uuid.UUID]struct{}, len(manifests))
	for _, manifest := range manifests {
		if manifest.ApplicationID == uuid.Nil {
			continue
		}
		if _, ok := seen[manifest.ApplicationID]; ok {
			continue
		}
		seen[manifest.ApplicationID] = struct{}{}
		application, err := h.apps.Get(ctx, manifest.ApplicationID)
		if err != nil || application == nil {
			continue
		}
		names[manifest.ApplicationID] = application.Name
	}
	return names
}

func buildManifestDoc(manifest manifestdomain.Manifest, applicationName string) ManifestDoc {
	return ManifestDoc{
		ID:                     manifest.ID,
		ApplicationID:          manifest.ApplicationID,
		ApplicationName:        applicationName,
		GitRevision:            manifest.GitRevision,
		RepoAddress:            manifest.RepoAddress,
		CommitHash:             manifest.CommitHash,
		ImageRef:               manifest.ImageRef,
		ImageTag:               manifest.ImageTag,
		ImageDigest:            manifest.ImageDigest,
		PipelineID:             manifest.PipelineID,
		TraceID:                manifest.TraceID,
		SpanID:                 manifest.SpanID,
		Steps:                  buildManifestStepDocs(manifest.Steps),
		ServicesSnapshot:       buildManifestServiceDocs(manifest.ServicesSnapshot),
		WorkloadConfigSnapshot: buildManifestWorkloadConfigDoc(manifest.WorkloadConfigSnapshot),
		Status:                 string(manifest.Status),
		CreatedAt:              manifest.CreatedAt.Format(time.RFC3339),
		UpdatedAt:              manifest.UpdatedAt.Format(time.RFC3339),
	}
}

func buildManifestStepDocs(steps []releasedomain.ImageTask) []ManifestStepDoc {
	if len(steps) == 0 {
		return nil
	}
	items := make([]ManifestStepDoc, 0, len(steps))
	for _, step := range steps {
		item := ManifestStepDoc{
			TaskName: step.TaskName,
			TaskRun:  step.TaskRun,
			Status:   string(step.Status),
			Message:  step.Message,
		}
		if step.StartTime != nil {
			item.StartTime = step.StartTime.Format(time.RFC3339)
		}
		if step.EndTime != nil {
			item.EndTime = step.EndTime.Format(time.RFC3339)
		}
		items = append(items, item)
	}
	return items
}

func buildManifestServiceDocs(services []manifestdomain.ManifestService) []ManifestServiceDoc {
	if len(services) == 0 {
		return nil
	}
	items := make([]ManifestServiceDoc, 0, len(services))
	for _, service := range services {
		ports := make([]ManifestServicePortDoc, 0, len(service.Ports))
		for _, port := range service.Ports {
			ports = append(ports, ManifestServicePortDoc{
				Name:        port.Name,
				ServicePort: port.ServicePort,
				TargetPort:  port.TargetPort,
				Protocol:    port.Protocol,
			})
		}
		items = append(items, ManifestServiceDoc{
			ID:    service.ID,
			Name:  service.Name,
			Ports: ports,
		})
	}
	return items
}

func buildManifestWorkloadConfigDoc(workload manifestdomain.ManifestWorkloadConfig) ManifestWorkloadConfigDoc {
	return ManifestWorkloadConfigDoc{
		ID:                 workload.ID,
		Replicas:           workload.Replicas,
		ServiceAccountName: workload.ServiceAccountName,
		Resources:          workload.Resources,
		Probes:             workload.Probes,
		Metrics:            workload.Metrics,
		EmptyDirs:          workload.EmptyDirs,
		Env:                buildManifestEnvVarDocs(workload.Env),
		Labels:             workload.Labels,
		Annotations:        workload.Annotations,
	}
}

func buildManifestEnvVarDocs(env []releasedomain.EnvVar) []ManifestEnvVarDoc {
	if len(env) == 0 {
		return nil
	}
	items := make([]ManifestEnvVarDoc, 0, len(env))
	for _, item := range env {
		items = append(items, ManifestEnvVarDoc{
			Name:  item.Name,
			Value: item.Value,
		})
	}
	return items
}
