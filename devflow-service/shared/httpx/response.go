package httpx

import "github.com/gin-gonic/gin"

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
