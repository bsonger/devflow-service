package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/google/uuid"
)

func TestMemoryStoreRuntimeSpecRevisionLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	applicationID := uuid.New()

	spec, err := store.EnsureRuntimeSpecByApplicationEnv(ctx, applicationID, "staging")
	if err != nil {
		t.Fatalf("EnsureRuntimeSpecByApplicationEnv() error = %v", err)
	}
	if spec == nil {
		t.Fatal("EnsureRuntimeSpecByApplicationEnv() returned nil spec")
	}

	fetched, err := store.GetRuntimeSpecByApplicationEnv(ctx, applicationID, "staging")
	if err != nil {
		t.Fatalf("GetRuntimeSpecByApplicationEnv() error = %v", err)
	}
	if fetched == nil || fetched.ID != spec.ID {
		t.Fatalf("GetRuntimeSpecByApplicationEnv() = %#v, want spec id %s", fetched, spec.ID)
	}

	nextRevision, err := store.NextRevisionNumber(ctx, spec.ID)
	if err != nil {
		t.Fatalf("NextRevisionNumber() error = %v", err)
	}
	if nextRevision != 1 {
		t.Fatalf("NextRevisionNumber() = %d, want 1", nextRevision)
	}

	revision := &runtimedomain.RuntimeSpecRevision{
		ID:               uuid.New(),
		RuntimeSpecID:    spec.ID,
		Revision:         nextRevision,
		Replicas:         2,
		HealthThresholds: `{"liveness":1}`,
		Resources:        `{"cpu":"100m"}`,
		Autoscaling:      `{"enabled":false}`,
		Scheduling:       `{"nodeSelector":{}}`,
		PodEnvs:          `[{"name":"APP_ENV","value":"staging"}]`,
		CreatedBy:        "tester",
		CreatedAt:        time.Now().UTC(),
	}
	if err := store.CreateRuntimeSpecRevision(ctx, revision); err != nil {
		t.Fatalf("CreateRuntimeSpecRevision() error = %v", err)
	}
	if err := store.UpdateCurrentRevision(ctx, spec.ID, revision.ID); err != nil {
		t.Fatalf("UpdateCurrentRevision() error = %v", err)
	}

	updatedSpec, err := store.GetRuntimeSpec(ctx, spec.ID)
	if err != nil {
		t.Fatalf("GetRuntimeSpec() error = %v", err)
	}
	if updatedSpec.CurrentRevisionID == nil || *updatedSpec.CurrentRevisionID != revision.ID {
		t.Fatalf("CurrentRevisionID = %v, want %s", updatedSpec.CurrentRevisionID, revision.ID)
	}

	revisions, err := store.ListRuntimeSpecRevisions(ctx, spec.ID)
	if err != nil {
		t.Fatalf("ListRuntimeSpecRevisions() error = %v", err)
	}
	if len(revisions) != 1 || revisions[0].ID != revision.ID {
		t.Fatalf("ListRuntimeSpecRevisions() = %#v, want revision id %s", revisions, revision.ID)
	}

	fetchedRevision, err := store.GetRuntimeSpecRevision(ctx, revision.ID)
	if err != nil {
		t.Fatalf("GetRuntimeSpecRevision() error = %v", err)
	}
	if fetchedRevision.CreatedBy != "tester" {
		t.Fatalf("GetRuntimeSpecRevision().CreatedBy = %q, want tester", fetchedRevision.CreatedBy)
	}
}

func TestMemoryStoreObservedWorkloadPodAndOperationLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	applicationID := uuid.New()
	spec, err := store.EnsureRuntimeSpecByApplicationEnv(ctx, applicationID, "production")
	if err != nil {
		t.Fatalf("EnsureRuntimeSpecByApplicationEnv() error = %v", err)
	}

	observedAt := time.Now().UTC().Add(-time.Minute)
	workload := &runtimedomain.RuntimeObservedWorkload{
		ID:                  uuid.New(),
		RuntimeSpecID:       spec.ID,
		ApplicationID:       applicationID,
		Environment:         "production",
		Namespace:           "devflow-prod",
		WorkloadKind:        "Deployment",
		WorkloadName:        "runtime-service",
		DesiredReplicas:     2,
		ReadyReplicas:       2,
		UpdatedReplicas:     2,
		AvailableReplicas:   2,
		UnavailableReplicas: 0,
		ObservedGeneration:  7,
		SummaryStatus:       "Healthy",
		Images:              []string{"repo/runtime-service:v1"},
		Conditions:          []runtimedomain.RuntimeObservedWorkloadCondition{{Type: "Available", Status: "True"}},
		Labels:              map[string]string{"app": "runtime-service"},
		Annotations:         map[string]string{"release": "r1"},
		ObservedAt:          observedAt,
	}
	if err := store.UpsertObservedWorkload(ctx, workload); err != nil {
		t.Fatalf("UpsertObservedWorkload() error = %v", err)
	}

	podA := &runtimedomain.RuntimeObservedPod{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: applicationID,
		Environment:   "production",
		Namespace:     "devflow-prod",
		PodName:       "runtime-service-1",
		Phase:         "Running",
		Ready:         true,
		Restarts:      0,
		ObservedAt:    observedAt,
		Labels:        map[string]string{"pod": "1"},
		Containers:    []runtimedomain.RuntimeObservedPodContainer{{Name: "runtime-service", Ready: true}},
	}
	podB := &runtimedomain.RuntimeObservedPod{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: applicationID,
		Environment:   "production",
		Namespace:     "devflow-prod",
		PodName:       "runtime-service-0",
		Phase:         "Running",
		Ready:         true,
		Restarts:      1,
		ObservedAt:    observedAt.Add(time.Second),
	}
	if err := store.UpsertObservedPod(ctx, podA); err != nil {
		t.Fatalf("UpsertObservedPod(podA) error = %v", err)
	}
	if err := store.UpsertObservedPod(ctx, podB); err != nil {
		t.Fatalf("UpsertObservedPod(podB) error = %v", err)
	}

	gotWorkload, err := store.GetObservedWorkload(ctx, spec.ID)
	if err != nil {
		t.Fatalf("GetObservedWorkload() error = %v", err)
	}
	if gotWorkload.WorkloadName != "runtime-service" {
		t.Fatalf("GetObservedWorkload().WorkloadName = %q, want runtime-service", gotWorkload.WorkloadName)
	}

	pods, err := store.ListObservedPods(ctx, spec.ID)
	if err != nil {
		t.Fatalf("ListObservedPods() error = %v", err)
	}
	if len(pods) != 2 {
		t.Fatalf("ListObservedPods() len = %d, want 2", len(pods))
	}
	if pods[0].PodName != "runtime-service-0" || pods[1].PodName != "runtime-service-1" {
		t.Fatalf("ListObservedPods() order = [%s %s], want [runtime-service-0 runtime-service-1]", pods[0].PodName, pods[1].PodName)
	}

	resolvedNamespace, err := store.ResolveTargetNamespace(ctx, applicationID, "production")
	if err != nil {
		t.Fatalf("ResolveTargetNamespace() error = %v", err)
	}
	if resolvedNamespace != "devflow-prod" {
		t.Fatalf("ResolveTargetNamespace() = %q, want devflow-prod", resolvedNamespace)
	}

	applicationName, err := store.GetApplicationName(ctx, applicationID)
	if err != nil {
		t.Fatalf("GetApplicationName() error = %v", err)
	}
	if applicationName != "runtime-service" {
		t.Fatalf("GetApplicationName() = %q, want runtime-service", applicationName)
	}

	op := &runtimedomain.RuntimeOperation{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		OperationType: "deployment_restart",
		TargetName:    "runtime-service",
		Operator:      "tester",
		CreatedAt:     observedAt.Add(2 * time.Second),
	}
	if err := store.CreateRuntimeOperation(ctx, op); err != nil {
		t.Fatalf("CreateRuntimeOperation() error = %v", err)
	}
	operations, err := store.ListRuntimeOperations(ctx, spec.ID)
	if err != nil {
		t.Fatalf("ListRuntimeOperations() error = %v", err)
	}
	if len(operations) != 1 || operations[0].TargetName != "runtime-service" {
		t.Fatalf("ListRuntimeOperations() = %#v", operations)
	}

	if err := store.DeleteObservedPod(ctx, spec.ID, "devflow-prod", "runtime-service-1", time.Now().UTC()); err != nil {
		t.Fatalf("DeleteObservedPod() error = %v", err)
	}
	pods, err = store.ListObservedPods(ctx, spec.ID)
	if err != nil {
		t.Fatalf("ListObservedPods() after delete error = %v", err)
	}
	if len(pods) != 1 || pods[0].PodName != "runtime-service-0" {
		t.Fatalf("ListObservedPods() after delete = %#v", pods)
	}

	if err := store.DeleteObservedWorkload(ctx, spec.ID, "devflow-prod", "Deployment", "runtime-service", time.Now().UTC()); err != nil {
		t.Fatalf("DeleteObservedWorkload() error = %v", err)
	}
	_, err = store.GetObservedWorkload(ctx, spec.ID)
	if err != sql.ErrNoRows {
		t.Fatalf("GetObservedWorkload() after delete error = %v, want %v", err, sql.ErrNoRows)
	}
}

func TestMemoryStoreDeleteRuntimeSpecCascadesAssociatedState(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	applicationID := uuid.New()
	spec, err := store.EnsureRuntimeSpecByApplicationEnv(ctx, applicationID, "qa")
	if err != nil {
		t.Fatalf("EnsureRuntimeSpecByApplicationEnv() error = %v", err)
	}

	if err := store.UpsertObservedWorkload(ctx, &runtimedomain.RuntimeObservedWorkload{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: applicationID,
		Environment:   "qa",
		Namespace:     "devflow-qa",
		WorkloadKind:  "Deployment",
		WorkloadName:  "runtime-service",
		ObservedAt:    time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedWorkload() error = %v", err)
	}
	if err := store.UpsertObservedPod(ctx, &runtimedomain.RuntimeObservedPod{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: applicationID,
		Environment:   "qa",
		Namespace:     "devflow-qa",
		PodName:       "runtime-service-0",
		Phase:         "Running",
		ObservedAt:    time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertObservedPod() error = %v", err)
	}
	if err := store.CreateRuntimeOperation(ctx, &runtimedomain.RuntimeOperation{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		OperationType: "pod_delete",
		TargetName:    "runtime-service-0",
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateRuntimeOperation() error = %v", err)
	}

	if err := store.DeleteRuntimeSpecByApplicationEnv(ctx, applicationID, "qa"); err != nil {
		t.Fatalf("DeleteRuntimeSpecByApplicationEnv() error = %v", err)
	}

	if _, err := store.GetRuntimeSpec(ctx, spec.ID); err != sql.ErrNoRows {
		t.Fatalf("GetRuntimeSpec() after delete error = %v, want %v", err, sql.ErrNoRows)
	}
	if item, err := store.GetRuntimeSpecByApplicationEnv(ctx, applicationID, "qa"); err != nil || item != nil {
		t.Fatalf("GetRuntimeSpecByApplicationEnv() after delete = %#v, %v; want nil, nil", item, err)
	}
	if _, err := store.GetObservedWorkload(ctx, spec.ID); err != sql.ErrNoRows {
		t.Fatalf("GetObservedWorkload() after cascade error = %v, want %v", err, sql.ErrNoRows)
	}
	pods, err := store.ListObservedPods(ctx, spec.ID)
	if err != nil {
		t.Fatalf("ListObservedPods() after cascade error = %v", err)
	}
	if len(pods) != 0 {
		t.Fatalf("ListObservedPods() after cascade len = %d, want 0", len(pods))
	}
	operations, err := store.ListRuntimeOperations(ctx, spec.ID)
	if err != nil {
		t.Fatalf("ListRuntimeOperations() after cascade error = %v", err)
	}
	if len(operations) != 0 {
		t.Fatalf("ListRuntimeOperations() after cascade len = %d, want 0", len(operations))
	}
}
