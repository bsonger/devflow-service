package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bsonger/devflow-service/internal/appconfig/domain"
	appconfigrepo "github.com/bsonger/devflow-service/internal/appconfig/repository"
	"github.com/bsonger/devflow-service/internal/platform/configrepo"
	"github.com/google/uuid"
)

type fakeSyncAppConfigStore struct {
	getItem         *domain.AppConfig
	latestRevision  *domain.AppConfigRevision
	inserted        *domain.AppConfigRevision
	updatedLatestID uuid.UUID
}

func (f *fakeSyncAppConfigStore) Create(context.Context, *domain.AppConfig) (uuid.UUID, error) {
	return uuid.Nil, errors.New("not implemented")
}
func (f *fakeSyncAppConfigStore) Get(context.Context, uuid.UUID) (*domain.AppConfig, error) {
	return f.getItem, nil
}
func (f *fakeSyncAppConfigStore) Update(context.Context, *domain.AppConfig) error { return nil }
func (f *fakeSyncAppConfigStore) Delete(context.Context, uuid.UUID) error         { return nil }
func (f *fakeSyncAppConfigStore) List(context.Context, appconfigrepo.AppConfigListFilter) ([]domain.AppConfig, error) {
	return nil, nil
}
func (f *fakeSyncAppConfigStore) GetLatestRevision(context.Context, uuid.UUID) (*domain.AppConfigRevision, error) {
	if f.latestRevision == nil {
		return nil, nil
	}
	return f.latestRevision, nil
}
func (f *fakeSyncAppConfigStore) GetRevision(context.Context, uuid.UUID) (*domain.AppConfigRevision, error) {
	return nil, nil
}
func (f *fakeSyncAppConfigStore) InsertRevision(_ context.Context, revision *domain.AppConfigRevision) error {
	f.inserted = revision
	return nil
}
func (f *fakeSyncAppConfigStore) UpdateLatestRevision(context.Context, uuid.UUID, int, uuid.UUID, time.Time) error {
	return nil
}
func (f *fakeSyncAppConfigStore) UpdateSourceDirectory(context.Context, uuid.UUID, string, time.Time) error {
	return nil
}

func TestSyncWithSnapshotRejectsForbiddenOtelFields(t *testing.T) {
	store := &fakeSyncAppConfigStore{
		getItem: &domain.AppConfig{BaseModel: domain.BaseModel{ID: uuid.New()}},
	}
	svc := &AppConfigService{store: store}
	cfg := &domain.AppConfig{BaseModel: domain.BaseModel{ID: uuid.New()}}
	snapshot := &configrepo.Snapshot{
		Files: []configrepo.File{{
			Name: "config.yaml",
			Content: "otel:\n" +
				"  endpoint: http://collector:4318\n" +
				"  service_name: config-service\n" +
				"  resource_attributes: service.version=v1,deployment.environment.name=production\n",
		}},
		SourceDigest: "digest-1",
	}

	_, err := svc.syncWithSnapshot(context.Background(), cfg, snapshot)
	if err == nil {
		t.Fatal("expected forbidden observability fields error")
	}
	if !errors.Is(err, ErrConfigObservabilityBoundary) {
		t.Fatalf("error = %v, want %v", err, ErrConfigObservabilityBoundary)
	}
	if store.inserted != nil {
		t.Fatalf("unexpected revision inserted: %#v", store.inserted)
	}
}

func TestSyncWithSnapshotAllowsEmptyLegacyResourceFields(t *testing.T) {
	store := &fakeSyncAppConfigStore{}
	svc := &AppConfigService{store: store}
	cfg := &domain.AppConfig{BaseModel: domain.BaseModel{ID: uuid.New()}}
	snapshot := &configrepo.Snapshot{
		Files: []configrepo.File{{
			Name: "config.yaml",
			Content: "otel:\n" +
				"  endpoint: http://collector:4318\n" +
				"  service_name: \"\"\n" +
				"  resource_attributes: \"\"\n",
		}},
		SourceDigest: "digest-2",
	}

	result, err := svc.syncWithSnapshot(context.Background(), cfg, snapshot)
	if err != nil {
		t.Fatalf("syncWithSnapshot returned error: %v", err)
	}
	if result == nil || result.Revision == nil {
		t.Fatal("expected created revision")
	}
	if store.inserted == nil {
		t.Fatal("expected revision to be inserted")
	}
}
