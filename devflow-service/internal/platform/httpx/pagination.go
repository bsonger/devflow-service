package httpx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const DefaultPageSize = 20
const MaxPageSize = 100

type Pagination struct {
	Enabled  bool
	Limit    int
	Offset   int
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

func ParsePagination(c *gin.Context) (Pagination, error) {
	var p Pagination

	pageStr := strings.TrimSpace(c.Query("page"))
	pageSizeStr := strings.TrimSpace(c.Query("page_size"))

	if pageStr != "" || pageSizeStr != "" {
		p.Enabled = true
	}

	if pageStr != "" || pageSizeStr != "" {
		page := 1
		if pageStr != "" {
			parsed, err := strconv.Atoi(pageStr)
			if err != nil || parsed < 1 {
				return Pagination{}, fmt.Errorf("invalid page")
			}
			page = parsed
		}

		pageSize := DefaultPageSize
		if pageSizeStr != "" {
			parsed, err := strconv.Atoi(pageSizeStr)
			if err != nil || parsed < 1 || parsed > MaxPageSize {
				return Pagination{}, fmt.Errorf("invalid page_size")
			}
			pageSize = parsed
		}

		p.Page = page
		p.PageSize = pageSize
		p.Limit = pageSize
		p.Offset = (page - 1) * pageSize
		return p, nil
	}

	return p, nil
}

func PaginateSlice[T any](items []T, p Pagination) []T {
	if !p.Enabled {
		return items
	}
	if p.Offset >= len(items) {
		return []T{}
	}

	end := p.Offset + p.Limit
	if end > len(items) {
		end = len(items)
	}
	return items[p.Offset:end]
}

func SetPaginationHeaders(c *gin.Context, total int, p Pagination) {
	if !p.Enabled {
		return
	}

	c.Header("X-Total-Count", strconv.Itoa(total))
	c.Header("X-Page", strconv.Itoa(p.Page))
	c.Header("X-Page-Size", strconv.Itoa(p.PageSize))
}

func IncludeDeleted(c *gin.Context) bool {
	return strings.EqualFold(strings.TrimSpace(c.Query("include_deleted")), "true")
}

func ParsePaginationOrWrite(c *gin.Context) (Pagination, bool) {
	paging, err := ParsePagination(c)
	if err != nil {
		_ = c.Error(err)
		WriteInvalidArgument(c, err.Error())
		return Pagination{}, false
	}

	return paging, true
}

func WritePaginatedList[T any](c *gin.Context, status int, items []T) bool {
	paging, ok := ParsePaginationOrWrite(c)
	if !ok {
		return false
	}

	total := len(items)
	items = PaginateSlice(items, paging)
	WriteList(c, status, items, paging, total)
	return true
}
