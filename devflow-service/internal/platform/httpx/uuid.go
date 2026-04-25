package httpx

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ParseUUID(raw, field string) (uuid.UUID, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return uuid.Nil, fmt.Errorf("invalid %s", field)
	}

	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s", field)
	}

	return id, nil
}

func ParseOptionalUUID(raw, field string) (*uuid.UUID, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}

	id, err := ParseUUID(value, field)
	if err != nil {
		return nil, err
	}

	return &id, nil
}

func ParseUUIDParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := ParseUUID(c.Param(name), name)
	if err != nil {
		WriteInvalidArgument(c, err.Error())
		return uuid.Nil, false
	}

	return id, true
}

func ParseUUIDQuery(c *gin.Context, name string) (*uuid.UUID, bool) {
	id, err := ParseOptionalUUID(c.Query(name), name)
	if err != nil {
		WriteInvalidArgument(c, err.Error())
		return nil, false
	}

	return id, true
}

func ParseUUIDString(c *gin.Context, value, field string) (uuid.UUID, bool) {
	id, err := ParseUUID(value, field)
	if err != nil {
		WriteInvalidArgument(c, err.Error())
		return uuid.Nil, false
	}

	return id, true
}
