package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	platformdb "github.com/bsonger/devflow-service/internal/platform/db"
	"github.com/bsonger/devflow-service/internal/workloadconfig/domain"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func setupWorkloadConfigRepositoryTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:workloadconfig-repo-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.Exec(`
CREATE TABLE workload_configs (
  id TEXT PRIMARY KEY,
  application_id TEXT NOT NULL,
  replicas INTEGER NOT NULL,
  service_account_name TEXT NULL,
  resources TEXT NOT NULL DEFAULT '{}',
  probes TEXT NOT NULL DEFAULT '{}',
  metrics TEXT NOT NULL DEFAULT '{}',
  empty_dirs TEXT NOT NULL DEFAULT '[]',
  env TEXT NOT NULL DEFAULT '[]',
  labels TEXT NOT NULL DEFAULT '{}',
  annotations TEXT NOT NULL DEFAULT '{}',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  deleted_at DATETIME NULL
);
`); err != nil {
		t.Fatalf("create workload_configs table: %v", err)
	}
	platformdb.InitPostgres(db)
	t.Cleanup(func() {
		_ = db.Close()
		platformdb.InitPostgres(nil)
	})
	return db
}

func insertLegacyWorkloadConfigRow(t *testing.T, db *sql.DB, rowID, appID uuid.UUID, resources, probes, metrics, emptyDirs, env, labels, annotations string) time.Time {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := db.Exec(`
		insert into workload_configs (
			id, application_id, replicas, service_account_name, resources, probes, metrics, empty_dirs, env, labels, annotations, created_at, updated_at, deleted_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,null)
	`, rowID.String(), appID.String(), 2, "runtime-service", resources, probes, metrics, emptyDirs, env, labels, annotations, now, now); err != nil {
		t.Fatalf("insert workload_config: %v", err)
	}
	return now
}

func TestGetMigratesLegacyRowToCanonicalContract(t *testing.T) {
	db := setupWorkloadConfigRepositoryTestDB(t)
	store := NewPostgresStore()
	id := uuid.New()
	appID := uuid.New()
	insertLegacyWorkloadConfigRow(
		t,
		db,
		id,
		appID,
		`{"requests":{"cpu":"250m","memory":"256Mi"},"limits":{"cpu":"1","memory":"1Gi"}}`,
		`{"liveness":{"path":"/healthz","port":"http","period_seconds":10},"readiness":{"path":"/readyz","port":"http","period_seconds":5}}`,
		`{"enabled":true,"port":9090}`,
		`[{"name":"tmp","mount_path":"/tmp"}]`,
		`[{"name":"LOG_LEVEL","value":"info"},{"name":"APP_MODE","value":"worker"}]`,
		`{"team":"platform"}`,
		`{"sidecar.istio.io/inject":"true"}`,
	)

	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Resources.SizeClass != "medium" {
		t.Fatalf("resources.size_class = %q, want medium", got.Resources.SizeClass)
	}
	if got.Resources.Requests.CPU != "250m" || got.Resources.Limits.Memory != "1Gi" {
		t.Fatalf("canonical resources not expanded: %#v", got.Resources)
	}
	if got.Probes.Liveness == nil || got.Probes.Liveness.Path != "/healthz" {
		t.Fatalf("liveness probe not preserved: %#v", got.Probes)
	}
	if !got.Metrics.Enabled || got.Metrics.Port != 9090 || got.Metrics.ScrapeProfile != "default" {
		t.Fatalf("metrics not normalized: %#v", got.Metrics)
	}
	if len(got.EmptyDirs) != 1 || got.EmptyDirs[0].Name != "tmp" || got.EmptyDirs[0].MountPath != "/tmp" {
		t.Fatalf("empty_dirs not preserved: %#v", got.EmptyDirs)
	}
	if len(got.Env) != 2 || got.Env[0].Name != "LOG_LEVEL" || got.Env[1].Name != "APP_MODE" {
		t.Fatalf("env order not preserved: %#v", got.Env)
	}

	var resourcesJSON, metricsJSON, emptyDirsJSON, envJSON string
	var deletedAt sql.NullTime
	if err := db.QueryRow(`select resources, metrics, empty_dirs, env, deleted_at from workload_configs where id=$1`, id.String()).Scan(&resourcesJSON, &metricsJSON, &emptyDirsJSON, &envJSON, &deletedAt); err != nil {
		t.Fatalf("select normalized row: %v", err)
	}
	if deletedAt.Valid {
		t.Fatal("expected migrated row to remain active")
	}
	wantResources := `{"size_class":"medium","requests":{"cpu":"250m","memory":"256Mi"},"limits":{"cpu":"1","memory":"1Gi"}}`
	if resourcesJSON != wantResources {
		t.Fatalf("normalized resources = %s, want %s", resourcesJSON, wantResources)
	}
	wantMetrics := `{"enabled":true,"port":9090,"scrape_profile":"default"}`
	if metricsJSON != wantMetrics {
		t.Fatalf("normalized metrics = %s, want %s", metricsJSON, wantMetrics)
	}
	wantEmptyDirs := `[{"name":"tmp","mount_path":"/tmp"}]`
	if emptyDirsJSON != wantEmptyDirs {
		t.Fatalf("normalized empty_dirs = %s, want %s", emptyDirsJSON, wantEmptyDirs)
	}
	wantEnv := `[{"name":"LOG_LEVEL","value":"info"},{"name":"APP_MODE","value":"worker"}]`
	if envJSON != wantEnv {
		t.Fatalf("normalized env = %s, want %s", envJSON, wantEnv)
	}
}

func TestGetSoftDeletesIncompatibleLegacyRow(t *testing.T) {
	db := setupWorkloadConfigRepositoryTestDB(t)
	store := NewPostgresStore()
	id := uuid.New()
	appID := uuid.New()
	insertLegacyWorkloadConfigRow(
		t,
		db,
		id,
		appID,
		`{"requests":{"cpu":"333m","memory":"256Mi"},"limits":{"cpu":"1","memory":"1Gi"}}`,
		`{"liveness":{"path":"/healthz","port":"http"}}`,
		`{}`,
		`[]`,
		`[{"name":"LOG_LEVEL","value":"info"}]`,
		`{"team":"platform"}`,
		`{"trace":"enabled"}`,
	)

	got, err := store.Get(context.Background(), id)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Get error = %v, want sql.ErrNoRows", err)
	}
	if got != nil {
		t.Fatalf("Get returned item %#v, want nil", got)
	}

	var deletedAt sql.NullTime
	if err := db.QueryRow(`select deleted_at from workload_configs where id=$1`, id.String()).Scan(&deletedAt); err != nil {
		t.Fatalf("select deleted row: %v", err)
	}
	if !deletedAt.Valid {
		t.Fatal("expected incompatible row to be soft-deleted")
	}

	got, err = store.Get(context.Background(), id)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("second Get error = %v, want sql.ErrNoRows", err)
	}
	if got != nil {
		t.Fatalf("second Get returned item %#v, want nil", got)
	}
}

func TestListOmitsRowsDeletedDuringNormalization(t *testing.T) {
	db := setupWorkloadConfigRepositoryTestDB(t)
	store := NewPostgresStore()
	appID := uuid.New()
	goodID := uuid.New()
	badID := uuid.New()
	insertLegacyWorkloadConfigRow(
		t,
		db,
		goodID,
		appID,
		`{"requests":{"cpu":"100m","memory":"128Mi"},"limits":{"cpu":"500m","memory":"512Mi"}}`,
		`{"startup":{"path":"/startupz","port":"http","period_seconds":5}}`,
		`{"enabled":true,"port":9090,"scrape_profile":"fast"}`,
		`[{"name":"tmp","mount_path":"/tmp"},{"name":"cache","mount_path":"/cache","medium":"Memory"}]`,
		`[]`,
		`{"team":"platform"}`,
		`{}`,
	)
	insertLegacyWorkloadConfigRow(
		t,
		db,
		badID,
		appID,
		`{"requests":{"cpu":"100m","memory":"128Mi"},"limits":{"cpu":"999m","memory":"512Mi"}}`,
		`{"readiness":{"path":"/readyz","port":"http"}}`,
		`{"enabled":true,"port":0}`,
		`[]`,
		`[{"name":"LOG_LEVEL","value":"info"},{"name":"LOG_LEVEL","value":"debug"}]`,
		`{"team":"platform"}`,
		`{}`,
	)

	items, err := store.List(context.Background(), ListFilter{ApplicationID: &appID})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("List len = %d, want 1", len(items))
	}
	if items[0].ID != goodID {
		t.Fatalf("List returned %s, want %s", items[0].ID, goodID)
	}
	if items[0].Resources.SizeClass != "small" {
		t.Fatalf("List size_class = %q, want small", items[0].Resources.SizeClass)
	}
	if items[0].Metrics.ScrapeProfile != "fast" {
		t.Fatalf("List scrape_profile = %q, want fast", items[0].Metrics.ScrapeProfile)
	}
	if len(items[0].EmptyDirs) != 2 || items[0].EmptyDirs[1].MountPath != "/cache" {
		t.Fatalf("List empty_dirs = %#v", items[0].EmptyDirs)
	}

	items, err = store.List(context.Background(), ListFilter{ApplicationID: &appID})
	if err != nil {
		t.Fatalf("second List returned error: %v", err)
	}
	if len(items) != 1 || items[0].ID != goodID {
		t.Fatalf("second List returned %#v, want only %s", items, goodID)
	}

	var deletedAt sql.NullTime
	if err := db.QueryRow(`select deleted_at from workload_configs where id=$1`, badID.String()).Scan(&deletedAt); err != nil {
		t.Fatalf("select deleted bad row: %v", err)
	}
	if !deletedAt.Valid {
		t.Fatal("expected invalid list row to be soft-deleted")
	}
}

func TestGetSoftDeletesMalformedJSONRow(t *testing.T) {
	db := setupWorkloadConfigRepositoryTestDB(t)
	store := NewPostgresStore()
	id := uuid.New()
	appID := uuid.New()
	insertLegacyWorkloadConfigRow(
		t,
		db,
		id,
		appID,
		`{"requests":{"cpu":"100m","memory":"128Mi"},`,
		`{"liveness":{"path":"/healthz","port":"http"}}`,
		`{}`,
		`[]`,
		`[{"name":"LOG_LEVEL","value":"info"}]`,
		`{"team":"platform"}`,
		`{}`,
	)

	got, err := store.Get(context.Background(), id)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Get error = %v, want sql.ErrNoRows", err)
	}
	if got != nil {
		t.Fatalf("Get returned item %#v, want nil", got)
	}

	var deletedAt sql.NullTime
	if err := db.QueryRow(`select deleted_at from workload_configs where id=$1`, id.String()).Scan(&deletedAt); err != nil {
		t.Fatalf("select malformed row: %v", err)
	}
	if !deletedAt.Valid {
		t.Fatal("expected malformed JSON row to be soft-deleted")
	}
}

func TestCreateAndGetPersistsEmptyDirs(t *testing.T) {
	db := setupWorkloadConfigRepositoryTestDB(t)
	store := NewPostgresStore()
	_ = db

	item := &domain.WorkloadConfig{
		ApplicationID: uuid.New(),
		Replicas:      1,
		Resources:     domain.WorkloadResourceRequirements{SizeClass: domain.WorkloadSizeClassSmall},
		EmptyDirs: []domain.WorkloadEmptyDir{
			{Name: "tmp", MountPath: "/tmp"},
			{Name: "cache", MountPath: "/cache"},
		},
	}
	item.WithCreateDefault()

	id, err := store.Create(context.Background(), item)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if len(got.EmptyDirs) != 2 {
		t.Fatalf("empty_dirs len = %d, want 2", len(got.EmptyDirs))
	}
	if got.EmptyDirs[0].Name != "tmp" || got.EmptyDirs[0].MountPath != "/tmp" {
		t.Fatalf("unexpected first empty_dir = %#v", got.EmptyDirs[0])
	}
	if got.EmptyDirs[1].Name != "cache" || got.EmptyDirs[1].MountPath != "/cache" {
		t.Fatalf("unexpected second empty_dir = %#v", got.EmptyDirs[1])
	}
}
