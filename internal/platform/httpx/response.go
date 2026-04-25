package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	internalErrorMessage      = "internal error"
	invalidRequestBodyMessage = "invalid request body"
)

type DataResponse[T any] struct {
	Data T `json:"data"`
}

type ListResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}

func WriteData[T any](c *gin.Context, status int, data T) {
	c.JSON(status, DataResponse[T]{Data: data})
}

func WriteList[T any](c *gin.Context, status int, items []T, pagination Pagination, total int) {
	pagination.Total = total
	c.JSON(status, ListResponse[T]{
		Data:       items,
		Pagination: pagination,
	})
}

func WriteNoContent(c *gin.Context) {
	c.Status(204)
	c.Writer.WriteHeaderNow()
}

func WriteError(c *gin.Context, status int, code, message string, details map[string]any) {
	c.JSON(status, ErrorResponse{
		Error: APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func WriteInvalidArgument(c *gin.Context, message string) {
	WriteError(c, http.StatusBadRequest, "invalid_argument", message, nil)
}

func WriteNotFound(c *gin.Context, message string) {
	WriteError(c, http.StatusNotFound, "not_found", message, nil)
}

func WriteConflict(c *gin.Context, message string) {
	WriteError(c, http.StatusConflict, "conflict", message, nil)
}

func WriteFailedPrecondition(c *gin.Context, status int, message string) {
	if status == 0 {
		status = http.StatusConflict
	}
	WriteError(c, status, "failed_precondition", message, nil)
}

func WriteUnauthorized(c *gin.Context) {
	WriteError(c, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
}

func WriteInternal(c *gin.Context) {
	WriteError(c, http.StatusInternalServerError, "internal", internalErrorMessage, nil)
}

func WriteInternalError(c *gin.Context, err error) {
	if err != nil {
		_ = c.Error(err)
	}
	WriteInternal(c)
}
