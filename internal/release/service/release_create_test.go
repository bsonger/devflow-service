package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	appconfigdownstream "github.com/bsonger/devflow-service/internal/appconfig/transport/downstream"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/release/repository"
	servicedownstream "github.com/bsonger/devflow-service/internal/service/transport/downstream"
	"github.com/google/uuid"
)

type stubReleaseManifestReader struct {
	getFn func(context.Context, uuid.UUID) (*manifestdomain.Manifest, error)
}

func (s stubReleaseManifestReader) Get(ctx context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
	return s.getFn(ctx, id)
}

type stubReleaseStore struct {
	getFn func(context.Context, uuid.UUID) (*model.Release, error)
}

func (s stubReleaseStore) Insert(context.Context, *model.Release) error { return nil }
func (s stubReleaseStore) Get(ctx context.Context, id uuid.UUID) (*model.Release, error) {
	return s.getFn(ctx, id)
}
func (s stubReleaseStore) Delete(context.Context, uuid.UUID) error { return nil }
func (s stubReleaseStore) List(context.Context, repository.ListFilter) ([]*model.Release, error) {
	return nil, nil
}
func (s stubReleaseStore) UpdateRow(context.Context, *model.Release) error   { return nil }
func (s stubReleaseStore) UpdateSteps(context.Context, *model.Release) error { return nil }
func (s stubReleaseStore) UpdateArgoMetadata(context.Context, uuid.UUID, string, string, time.Time) error {
	return nil
}

type stubReleaseBundleStore struct{}

func (stubReleaseBundleStore) Insert(context.Context, *model.ReleaseBundleRecord) error { return nil }
func (stubReleaseBundleStore) GetByReleaseID(context.Context, uuid.UUID) (*model.ReleaseBundleRecord, error) {
	return nil, sql.ErrNoRows
}

type stubReleaseConfigReader struct {
	findFn func(context.Context, string, string) (*appconfigdownstream.AppConfig, error)
}

func (s stubReleaseConfigReader) FindAppConfig(ctx context.Context, applicationID, environmentID string) (*appconfigdownstream.AppConfig, error) {
	return s.findFn(ctx, applicationID, environmentID)
}

type stubReleaseNetworkReader struct {
	listFn func(context.Context, string, string) ([]servicedownstream.Route, error)
}

func (s stubReleaseNetworkReader) ListRoutes(ctx context.Context, applicationID, environmentID string) ([]servicedownstream.Route, error) {
	return s.listFn(ctx, applicationID, environmentID)
}

func TestPopulateReleaseDefaultsPreservesProvidedEnv(t *testing.T) {
	manifestID := uuid.New()
	appID := uuid.New()
	release := &model.Release{ManifestID: manifestID, EnvironmentID: "staging"}

	populateReleaseDefaults(release, appID, "prod")

	if release.EnvironmentID != "staging" {
		t.Fatalf("got env %s want staging", release.EnvironmentID)
	}
	if release.Strategy != string(model.ReleaseStrategyRolling) {
		t.Fatalf("got strategy %s want %s", release.Strategy, model.ReleaseStrategyRolling)
	}
}

func TestPopulateReleaseDefaultsFallsBackToProd(t *testing.T) {
	manifestID := uuid.New()
	appID := uuid.New()
	release := &model.Release{ManifestID: manifestID}

	populateReleaseDefaults(release, appID, "prod")

	if release.EnvironmentID != "prod" {
		t.Fatalf("got env %s want prod", release.EnvironmentID)
	}
	if release.Type != model.ReleaseUpgrade {
		t.Fatalf("got type %s want %s", release.Type, model.ReleaseUpgrade)
	}
	if release.Strategy != string(model.ReleaseStrategyRolling) {
		t.Fatalf("got strategy %s want %s", release.Strategy, model.ReleaseStrategyRolling)
	}
}

func TestPopulateReleaseDefaultsPreservesManifestID(t *testing.T) {
	manifestID := uuid.New()
	appID := uuid.New()
	release := &model.Release{ManifestID: manifestID}

	populateReleaseDefaults(release, appID, "prod")

	if release.ManifestID != manifestID {
		t.Fatalf("got manifest id %s want %s", release.ManifestID, manifestID)
	}
}

func TestCreateReleaseRejectsManifestThatIsNotAvailable(t *testing.T) {
	originalManifestSource := releaseManifestSource
	releaseManifestSource = stubReleaseManifestReader{
		getFn: func(_ context.Context, id uuid.UUID) (*manifestdomain.Manifest, error) {
			return &manifestdomain.Manifest{
				BaseModel:     model.BaseModel{ID: id},
				ApplicationID: uuid.New(),
				Status:        model.ManifestPending,
			}, nil
		},
	}
	defer func() { releaseManifestSource = originalManifestSource }()

	svc := &releaseService{}
	_, err := svc.Create(context.Background(), &model.Release{
		ManifestID: uuid.New(),
		Type:       model.ReleaseUpgrade,
	})
	if err == nil {
		t.Fatalf("expected manifest not available error")
	}
	if err != ErrReleaseManifestNotAvailable {
		t.Fatalf("got err %v want %v", err, ErrReleaseManifestNotAvailable)
	}
}

func TestFreezeReleaseLiveInputsAllowsMissingAppConfig(t *testing.T) {
	originalConfigFactory := releaseConfigReaderFactory
	originalNetworkFactory := releaseNetworkReaderFactory
	defer func() {
		releaseConfigReaderFactory = originalConfigFactory
		releaseNetworkReaderFactory = originalNetworkFactory
	}()

	releaseConfigReaderFactory = func() releaseConfigReader {
		return stubReleaseConfigReader{
			findFn: func(_ context.Context, applicationID, environmentID string) (*appconfigdownstream.AppConfig, error) {
				if applicationID == "" || environmentID == "" {
					t.Fatalf("unexpected lookup args: application=%q environment=%q", applicationID, environmentID)
				}
				return nil, nil
			},
		}
	}
	releaseNetworkReaderFactory = func() releaseNetworkReader {
		return stubReleaseNetworkReader{
			listFn: func(_ context.Context, applicationID, environmentID string) ([]servicedownstream.Route, error) {
				return []servicedownstream.Route{{
					ID:          uuid.New().String(),
					Name:        "api",
					Host:        "document.example.com",
					Path:        "/",
					ServiceName: "document",
					ServicePort: 80,
				}}, nil
			},
		}
	}

	release := &model.Release{
		ApplicationID: uuid.New(),
		EnvironmentID: "production",
	}
	if err := freezeReleaseLiveInputs(context.Background(), release); err != nil {
		t.Fatalf("freezeReleaseLiveInputs failed: %v", err)
	}
	if len(release.AppConfigSnapshot.Files) != 0 || len(release.AppConfigSnapshot.Data) != 0 {
		t.Fatalf("expected empty app config snapshot, got %#v", release.AppConfigSnapshot)
	}
	if len(release.RoutesSnapshot) != 1 {
		t.Fatalf("expected one route snapshot, got %#v", release.RoutesSnapshot)
	}
}

func TestIsReleaseDeployableManifestStatus(t *testing.T) {
	if !isReleaseDeployableManifestStatus(model.ManifestAvailable) {
		t.Fatal("ManifestAvailable should be deployable")
	}
	if isReleaseDeployableManifestStatus(model.ManifestUnavailable) {
		t.Fatal("ManifestUnavailable should not be deployable")
	}
	if isReleaseDeployableManifestStatus(model.ManifestRunning) {
		t.Fatal("ManifestRunning should not be deployable")
	}
}

func TestReleaseTargetEnvironmentUsesReleaseEnvironmentOnly(t *testing.T) {
	release := &model.Release{EnvironmentID: "  staging "}

	got := releaseTargetEnvironment(release)

	if got != "staging" {
		t.Fatalf("releaseTargetEnvironment() = %q want staging", got)
	}
}

func TestSelectReleaseRoutesIncludesBaseAndTargetEnvironment(t *testing.T) {
	routes := []servicedownstream.Route{
		{Name: "base", EnvironmentID: "base"},
		{Name: "target", EnvironmentID: "staging"},
		{Name: "other", EnvironmentID: "prod"},
		{Name: "empty", EnvironmentID: ""},
	}

	got := selectReleaseRoutes(routes, "staging")

	if len(got) != 3 {
		t.Fatalf("len(selectReleaseRoutes) = %d want 3", len(got))
	}
	if got[0].Name != "base" || got[1].Name != "target" || got[2].Name != "empty" {
		t.Fatalf("unexpected selected routes: %+v", got)
	}
}

func TestPopulateReleaseDefaultsNormalizesLegacyDeployType(t *testing.T) {
	manifestID := uuid.New()
	appID := uuid.New()
	release := &model.Release{ManifestID: manifestID, Type: "deploy"}

	populateReleaseDefaults(release, appID, "prod")

	if release.Type != model.ReleaseUpgrade {
		t.Fatalf("got type %s want %s", release.Type, model.ReleaseUpgrade)
	}
}

func TestPopulateReleaseDefaultsNormalizesLowercaseRollbackType(t *testing.T) {
	manifestID := uuid.New()
	appID := uuid.New()
	release := &model.Release{ManifestID: manifestID, Type: "rollback"}

	populateReleaseDefaults(release, appID, "prod")

	if release.Type != model.ReleaseRollback {
		t.Fatalf("got type %s want %s", release.Type, model.ReleaseRollback)
	}
}

func TestLoadReleaseNormalizesLegacyTypeFromStore(t *testing.T) {
	releaseID := uuid.New()
	svc := &releaseService{
		store: stubReleaseStore{
			getFn: func(_ context.Context, id uuid.UUID) (*model.Release, error) {
				if id != releaseID {
					t.Fatalf("got id %s want %s", id, releaseID)
				}
				return &model.Release{
					BaseModel:     model.BaseModel{ID: id},
					ApplicationID: uuid.New(),
					ManifestID:    uuid.New(),
					EnvironmentID: "prod",
					Strategy:      "",
					Type:          "deploy",
				}, nil
			},
		},
		bundleStore: stubReleaseBundleStore{},
	}

	release, err := svc.loadRelease(context.Background(), releaseID)
	if err != nil {
		t.Fatalf("loadRelease() error = %v", err)
	}
	if release.Type != model.ReleaseUpgrade {
		t.Fatalf("got type %s want %s", release.Type, model.ReleaseUpgrade)
	}
	if release.Strategy != string(model.ReleaseStrategyRolling) {
		t.Fatalf("got strategy %s want %s", release.Strategy, model.ReleaseStrategyRolling)
	}
}
