package service

import (
	"context"
	"testing"
	"time"

	"github.com/bsonger/devflow-service/internal/appconfig/domain"
	appconfigrepo "github.com/bsonger/devflow-service/internal/appconfig/repository"
	"github.com/google/uuid"
)

type fakeAppConfigStore struct {
	listItems  []domain.AppConfig
	revisions  map[uuid.UUID]*domain.AppConfigRevision
	listCalled bool
}

func (f *fakeAppConfigStore) Create(context.Context, *domain.AppConfig) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (f *fakeAppConfigStore) Get(context.Context, uuid.UUID) (*domain.AppConfig, error) {
	return nil, nil
}
func (f *fakeAppConfigStore) Update(context.Context, *domain.AppConfig) error { return nil }
func (f *fakeAppConfigStore) Delete(context.Context, uuid.UUID) error         { return nil }
func (f *fakeAppConfigStore) List(context.Context, appconfigrepo.AppConfigListFilter) ([]domain.AppConfig, error) {
	f.listCalled = true
	return f.listItems, nil
}
func (f *fakeAppConfigStore) GetLatestRevision(context.Context, uuid.UUID) (*domain.AppConfigRevision, error) {
	return nil, nil
}
func (f *fakeAppConfigStore) GetRevision(_ context.Context, id uuid.UUID) (*domain.AppConfigRevision, error) {
	if item, ok := f.revisions[id]; ok {
		return item, nil
	}
	return nil, nil
}
func (f *fakeAppConfigStore) InsertRevision(context.Context, *domain.AppConfigRevision) error {
	return nil
}
func (f *fakeAppConfigStore) UpdateLatestRevision(context.Context, uuid.UUID, int, uuid.UUID, time.Time) error {
	return nil
}
func (f *fakeAppConfigStore) UpdateSourceDirectory(context.Context, uuid.UUID, string, time.Time) error {
	return nil
}

func TestAppConfigServiceListHydratesFilesFromLatestRevision(t *testing.T) {
	revisionID := uuid.New()
	store := &fakeAppConfigStore{
		listItems: []domain.AppConfig{{
			BaseModel:        domain.BaseModel{ID: uuid.New()},
			ApplicationID:    uuid.New(),
			EnvironmentID:    "env-1",
			LatestRevisionID: &revisionID,
		}},
		revisions: map[uuid.UUID]*domain.AppConfigRevision{
			revisionID: {
				ID:           revisionID,
				Files:        []domain.File{{Name: "config.yaml", Content: "a: b\n"}},
				SourceCommit: "abc123",
			},
		},
	}
	svc := &AppConfigService{store: store}

	items, err := svc.List(context.Background(), AppConfigListFilter{EnvironmentID: "env-1"})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if !store.listCalled {
		t.Fatal("expected store List to be called")
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d", len(items))
	}
	if len(items[0].Files) != 1 || items[0].Files[0].Name != "config.yaml" {
		t.Fatalf("files = %+v", items[0].Files)
	}
	if items[0].SourceCommit != "abc123" {
		t.Fatalf("source_commit = %q", items[0].SourceCommit)
	}
}
