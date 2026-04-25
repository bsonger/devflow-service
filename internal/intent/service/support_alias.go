package service

import (
	"database/sql"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/dbsql"
	intentdomain "github.com/bsonger/devflow-service/internal/intent/domain"
)

func marshalJSON(value any, empty string) ([]byte, error) {
	return dbsql.MarshalJSON(value, empty)
}

func nullableTimePtr(t *time.Time) any {
	return dbsql.NullableTimePtr(t)
}

func placeholderClause(column string, position int) string {
	return dbsql.PlaceholderClause(column, position)
}

func ensureRowsAffected(result sql.Result) error {
	return dbsql.EnsureRowsAffected(result)
}

func scanIntent(scanner interface{ Scan(dest ...any) error }) (*intentdomain.Intent, error) {
	return ScanIntent(scanner)
}
