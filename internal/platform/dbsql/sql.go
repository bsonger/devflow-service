package dbsql

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func MarshalJSON(value any, empty string) ([]byte, error) {
	if value == nil {
		return []byte(empty), nil
	}
	return json.Marshal(value)
}

func MustMarshalJSON(value any, empty string) []byte {
	payload, err := MarshalJSON(value, empty)
	if err != nil {
		return []byte(empty)
	}
	return payload
}

func NullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func NullableUUIDPtr(id *uuid.UUID) any {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	return *id
}

func NullableTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func EmptyToNull(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func TimePtrFromNull(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	parsed := value.Time
	return &parsed
}

func PlaceholderClause(column string, position int) string {
	return column + " = $" + strconv.Itoa(position)
}

func ParseNullUUID(value sql.NullString) (*uuid.UUID, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func EnsureRowsAffected(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
