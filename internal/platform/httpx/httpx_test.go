package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func TestWriteInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	WriteInternalError(ctx, assertErr("boom"))

	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if body.Error.Code != "internal" || body.Error.Message != "internal error" {
		t.Fatalf("unexpected error payload: %#v", body.Error)
	}
}

func TestWriteUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	WriteUnauthorized(ctx)

	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if body.Error.Code != "unauthorized" || body.Error.Message != "unauthorized" {
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

func TestParsePaginationOrWriteRejectsInvalidValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?page=bad", nil)

	_, ok := ParsePaginationOrWrite(ctx)
	if ok {
		t.Fatal("expected pagination parse to fail")
	}

	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Error.Code != "invalid_argument" || body.Error.Message != "invalid page" {
		t.Fatalf("unexpected error payload: %#v", body.Error)
	}
}

func TestWritePaginatedList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?page=2&page_size=1", nil)

	ok := WritePaginatedList(ctx, http.StatusOK, []gin.H{{"id": "a"}, {"id": "b"}})
	if !ok {
		t.Fatal("expected paginated write to succeed")
	}

	var body struct {
		Data       []map[string]string `json:"data"`
		Pagination Pagination          `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0]["id"] != "b" {
		t.Fatalf("unexpected data payload: %#v", body.Data)
	}
	if body.Pagination.Total != 2 || body.Pagination.Page != 2 || body.Pagination.PageSize != 1 {
		t.Fatalf("unexpected pagination payload: %#v", body.Pagination)
	}
}

func TestBindJSONRejectsInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", httptest.NewRecorder().Body)

	var payload struct {
		Name string `json:"name"`
	}
	ok := BindJSON(ctx, &payload)
	if ok {
		t.Fatal("expected bind to fail")
	}

	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Error.Code != "invalid_argument" || body.Error.Message != "invalid request body" {
		t.Fatalf("unexpected error payload: %#v", body.Error)
	}
}

func TestParseUUIDParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, router := gin.CreateTestContext(rec)
	router.GET("/:id", func(c *gin.Context) {
		id, ok := ParseUUIDParam(c, "id")
		if !ok {
			return
		}
		WriteData(c, http.StatusOK, gin.H{"id": id.String()})
	})

	validID := uuid.NewString()
	ctx.Request = httptest.NewRequest(http.MethodGet, "/"+validID, nil)
	router.HandleContext(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["data"]["id"] != validID {
		t.Fatalf("data.id = %q, want %q", body["data"]["id"], validID)
	}
}

func TestParseUUIDParamRejectsInvalidValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, router := gin.CreateTestContext(rec)
	router.GET("/:id", func(c *gin.Context) {
		_, _ = ParseUUIDParam(c, "id")
	})

	ctx.Request = httptest.NewRequest(http.MethodGet, "/bad-id", nil)
	router.HandleContext(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Error.Code != "invalid_argument" || body.Error.Message != "invalid id" {
		t.Fatalf("unexpected error payload: %#v", body.Error)
	}
}

func TestParseUUIDQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	validID := uuid.NewString()
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?application_id="+validID, nil)

	id, ok := ParseUUIDQuery(ctx, "application_id")
	if !ok {
		t.Fatal("expected query parse to succeed")
	}
	if id == nil || id.String() != validID {
		t.Fatalf("id = %#v, want %q", id, validID)
	}
}

func TestParseUUIDQueryEmptyValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	id, ok := ParseUUIDQuery(ctx, "application_id")
	if !ok {
		t.Fatal("expected empty query parse to succeed")
	}
	if id != nil {
		t.Fatalf("id = %#v, want nil", id)
	}
}

func TestParseUUIDStringRejectsInvalidValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	_, ok := ParseUUIDString(ctx, "bad-id", "release_id")
	if ok {
		t.Fatal("expected parse to fail")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Error.Code != "invalid_argument" || body.Error.Message != "invalid release_id" {
		t.Fatalf("unexpected error payload: %#v", body.Error)
	}
}

type staticErr string

func (e staticErr) Error() string { return string(e) }

func assertErr(message string) error {
	return staticErr(message)
}
