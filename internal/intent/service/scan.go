package service

import (
	"database/sql"

	intentdomain "github.com/bsonger/devflow-service/internal/intent/domain"
)

func ScanIntent(scanner interface {
	Scan(dest ...any) error
}) (*intentdomain.Intent, error) {
	var (
		item           intentdomain.Intent
		claimedAt      sql.NullTime
		leaseExpiresAt sql.NullTime
		deletedAt      sql.NullTime
	)

	if err := scanner.Scan(
		&item.ID,
		&item.Kind,
		&item.Status,
		&item.ResourceType,
		&item.ResourceID,
		&item.TraceID,
		&item.Message,
		&item.LastError,
		&item.ClaimedBy,
		&claimedAt,
		&leaseExpiresAt,
		&item.AttemptCount,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	if claimedAt.Valid {
		item.ClaimedAt = &claimedAt.Time
	}
	if leaseExpiresAt.Valid {
		item.LeaseExpiresAt = &leaseExpiresAt.Time
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
	return &item, nil
}
