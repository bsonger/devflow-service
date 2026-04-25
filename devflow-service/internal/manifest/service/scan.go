package service

import (
	"database/sql"
	"encoding/json"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
)

func ScanManifest(scanner interface {
	Scan(dest ...any) error
}) (*manifestdomain.Manifest, error) {
	var (
		item                manifestdomain.Manifest
		artifactPushedAt    sql.NullTime
		servicesJSON        []byte
		routesJSON          []byte
		appConfigJSON       []byte
		workloadConfigJSON  []byte
		renderedObjectsJSON []byte
		deletedAt           sql.NullTime
	)
	if err := scanner.Scan(
		&item.ID,
		&item.ApplicationID,
		&item.EnvironmentID,
		&item.ImageID,
		&item.ImageRef,
		&item.ArtifactRepository,
		&item.ArtifactTag,
		&item.ArtifactRef,
		&item.ArtifactDigest,
		&item.ArtifactMediaType,
		&artifactPushedAt,
		&servicesJSON,
		&routesJSON,
		&appConfigJSON,
		&workloadConfigJSON,
		&renderedObjectsJSON,
		&item.RenderedYAML,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	if len(servicesJSON) > 0 {
		if err := json.Unmarshal(servicesJSON, &item.ServicesSnapshot); err != nil {
			return nil, err
		}
	}
	if len(routesJSON) > 0 {
		if err := json.Unmarshal(routesJSON, &item.RoutesSnapshot); err != nil {
			return nil, err
		}
	}
	if len(appConfigJSON) > 0 {
		if err := json.Unmarshal(appConfigJSON, &item.AppConfigSnapshot); err != nil {
			return nil, err
		}
	}
	if len(workloadConfigJSON) > 0 {
		if err := json.Unmarshal(workloadConfigJSON, &item.WorkloadConfigSnapshot); err != nil {
			return nil, err
		}
	}
	if len(renderedObjectsJSON) > 0 {
		if err := json.Unmarshal(renderedObjectsJSON, &item.RenderedObjects); err != nil {
			return nil, err
		}
	}
	if artifactPushedAt.Valid {
		item.ArtifactPushedAt = &artifactPushedAt.Time
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
	return &item, nil
}
