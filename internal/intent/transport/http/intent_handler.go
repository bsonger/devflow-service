package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	intentdomain "github.com/bsonger/devflow-service/internal/intent/domain"
	intentservice "github.com/bsonger/devflow-service/internal/intent/service"
	"github.com/bsonger/devflow-service/internal/platform/httpx"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type intentService interface {
	Get(ctx context.Context, id uuid.UUID) (*intentdomain.Intent, error)
	List(ctx context.Context, filter intentservice.IntentListFilter) ([]*intentdomain.Intent, error)
}

type IntentHandler struct {
	svc intentService
}

type IntentResponse struct {
	Data *IntentDoc `json:"data"`
}

type IntentListResponse struct {
	Data       []*IntentDoc     `json:"data"`
	Pagination httpx.Pagination `json:"pagination"`
}

func NewIntentHandler(svc intentService) *IntentHandler {
	return &IntentHandler{svc: svc}
}

func (h *IntentHandler) RegisterRoutes(rg *gin.RouterGroup) {
	intent := rg.Group("/intents")
	intent.GET("", h.List)
	intent.GET("/:id", h.Get)
}

// List
// @Summary 获取执行意图列表
// @Description 按 kind、status、resource 等维度查询 execution intents
// @Tags Intent
// @Success 200 {object} IntentListResponse
// @Router /api/v1/intents [get]
func (h *IntentHandler) List(c *gin.Context) {
	filter, err := buildIntentFilter(c)
	if err != nil {
		httpx.WriteInvalidArgument(c, err.Error())
		return
	}

	intents, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteInternalError(c, err)
		return
	}
	httpx.WritePaginatedList(c, http.StatusOK, intents)
}

// Get
// @Summary 获取执行意图
// @Tags Intent
// @Param id path string true "Intent ID"
// @Success 200 {object} IntentResponse
// @Router /api/v1/intents/{id} [get]
func (h *IntentHandler) Get(c *gin.Context) {
	id, ok := httpx.ParseUUIDParam(c, "id")
	if !ok {
		return
	}

	intent, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(c, "not found")
			return
		}
		httpx.WriteInternalError(c, err)
		return
	}

	httpx.WriteData(c, http.StatusOK, intent)
}

func buildIntentFilter(c *gin.Context) (intentservice.IntentListFilter, error) {
	filter := intentservice.IntentListFilter{}

	if kind := strings.TrimSpace(c.Query("kind")); kind != "" {
		filter.Kind = string(model.IntentKind(kind))
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		filter.Status = string(model.IntentStatus(status))
	}
	if resourceType := strings.TrimSpace(c.Query("resource_type")); resourceType != "" {
		filter.ResourceType = resourceType
	}
	if claimedBy := strings.TrimSpace(c.Query("claimed_by")); claimedBy != "" {
		filter.ClaimedBy = claimedBy
	}

	resourceID, err := httpx.ParseOptionalUUID(c.Query("resource_id"), "resource_id")
	if err != nil {
		return intentservice.IntentListFilter{}, err
	}
	filter.ResourceID = resourceID

	return filter, nil
}
