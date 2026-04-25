package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWriteData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	WriteData(ctx, http.StatusCreated, gin.H{"id": "123"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var body map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["data"]["id"] != "123" {
		t.Fatalf("data.id = %q, want %q", body["data"]["id"], "123")
	}
}

func TestWriteList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	WriteList(ctx, http.StatusOK, []gin.H{{"id": "a"}}, Pagination{Page: 2, PageSize: 10}, 21)

	var body struct {
		Data       []map[string]string `json:"data"`
		Pagination Pagination          `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0]["id"] != "a" {
		t.Fatalf("unexpected data payload: %#v", body.Data)
	}
	if body.Pagination.Page != 2 || body.Pagination.PageSize != 10 || body.Pagination.Total != 21 {
		t.Fatalf("unexpected pagination payload: %#v", body.Pagination)
	}
}

func TestWriteNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	WriteNoContent(ctx)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestWriteError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	WriteError(ctx, http.StatusBadRequest, "invalid_argument", "invalid page", map[string]any{"field": "page"})

	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Error.Code != "invalid_argument" || body.Error.Message != "invalid page" {
		t.Fatalf("unexpected error payload: %#v", body.Error)
	}
}

func TestParsePaginationRejectsTooLargePageSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?page=1&page_size=101", nil)

	_, err := ParsePagination(ctx)
	if err == nil || err.Error() != "invalid page_size" {
		t.Fatalf("unexpected error: %v", err)
	}
}
