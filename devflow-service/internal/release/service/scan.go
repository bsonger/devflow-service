package service

import (
	"database/sql"
	"encoding/json"

	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
)

func ScanRelease(scanner interface {
	Scan(dest ...any) error
}) (*model.Release, error) {
	var (
		item            model.Release
		executionIntent sql.NullString
		stepsBytes      []byte
		deletedAt       sql.NullTime
	)

	if err := scanner.Scan(
		&item.ID,
		&executionIntent,
		&item.ApplicationID,
		&item.ManifestID,
		&item.ImageID,
		&item.Env,
		&item.Type,
		&stepsBytes,
		&item.Status,
		&item.ExternalRef,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}

	var err error
	item.ExecutionIntentID, err = dbsql.ParseNullUUID(executionIntent)
	if err != nil {
		return nil, err
	}
	if len(stepsBytes) > 0 {
		if err := json.Unmarshal(stepsBytes, &item.Steps); err != nil {
			return nil, err
		}
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
	return &item, nil
}
