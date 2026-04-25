package service

import (
	"database/sql"
	"encoding/json"

	imagedomain "github.com/bsonger/devflow-service/internal/image/domain"
	"github.com/bsonger/devflow-service/internal/platform/dbsql"
)

func ScanImage(scanner interface {
	Scan(dest ...any) error
}) (*imagedomain.Image, error) {
	var (
		item                    imagedomain.Image
		executionIntent         sql.NullString
		configurationRevisionID sql.NullString
		runtimeSpecRevisionID   sql.NullString
		stepsBytes              []byte
		deletedAt               sql.NullTime
	)

	if err := scanner.Scan(
		&item.ID,
		&executionIntent,
		&item.ApplicationID,
		&configurationRevisionID,
		&runtimeSpecRevisionID,
		&item.Name,
		&item.Tag,
		&item.Branch,
		&item.RepoAddress,
		&item.CommitHash,
		&item.Digest,
		&item.PipelineID,
		&stepsBytes,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}

	intentID, err := dbsql.ParseNullUUID(executionIntent)
	if err != nil {
		return nil, err
	}
	item.ExecutionIntentID = intentID
	item.ConfigurationRevisionID, err = dbsql.ParseNullUUID(configurationRevisionID)
	if err != nil {
		return nil, err
	}
	item.RuntimeSpecRevisionID, err = dbsql.ParseNullUUID(runtimeSpecRevisionID)
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
