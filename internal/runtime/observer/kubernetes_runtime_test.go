package observer

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	platformobserver "github.com/bsonger/devflow-service/internal/platform/observer"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	runtimerepo "github.com/bsonger/devflow-service/internal/runtime/repository"
	runtimeservice "github.com/bsonger/devflow-service/internal/runtime/service"
	"github.com/google/uuid"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

func TestKubernetesRuntimeObserverLogsObservedStateTransitions(t *testing.T) {
	appID := uuid.New()
	specID := uuid.New()
	store := runtimerepo.NewMemoryStore()
	service := runtimeservice.New(store, nil)
	observer := &KubernetesRuntimeObserver{
		store:   store,
		runtime: service,
	}
	spec := &runtimedomain.RuntimeSpec{
		ID:            specID,
		ApplicationID: appID,
		Environment:   "prod",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := store.CreateRuntimeSpec(context.Background(), spec); err != nil {
		t.Fatalf("CreateRuntimeSpec failed: %v", err)
	}

	var mu sync.Mutex
	var events []string
	runtimeObserverLogf = func(event string, fields ...zap.Field) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	}
	defer func() { runtimeObserverLogf = defaultRuntimeObserverLogf }()

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-api",
			Namespace: "devflow",
			Labels: map[string]string{
				releasedomain.ReleaseApplicationLabel: appID.String(),
				releasedomain.ReleaseEnvironmentLabel: "prod",
				releasedomain.ReleaseIDLabel:          uuid.New().String(),
				releasedomain.ReleaseStatusLabel:      string(releasedomain.ReleaseRunning),
				platformobserver.ObserveStateLabel:    platformobserver.ObserveStateRunning,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32PtrForObserverTest(2),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas:       1,
			UpdatedReplicas:     1,
			AvailableReplicas:   1,
			UnavailableReplicas: 1,
			ObservedGeneration:  1,
		},
	}

	if err := observer.syncWorkload(context.Background(), spec, "demo-api", "devflow", []appsv1.Deployment{deployment}, nil); err != nil {
		t.Fatalf("first syncWorkload failed: %v", err)
	}
	if err := observer.syncWorkload(context.Background(), spec, "demo-api", "devflow", []appsv1.Deployment{deployment}, nil); err != nil {
		t.Fatalf("second syncWorkload failed: %v", err)
	}

	deployment.Status.ReadyReplicas = 2
	deployment.Status.UpdatedReplicas = 2
	deployment.Status.AvailableReplicas = 2
	deployment.Status.UnavailableReplicas = 0
	deployment.Labels[platformobserver.ObserveStateLabel] = platformobserver.ObserveStateDone
	deployment.Labels[releasedomain.ReleaseStatusLabel] = string(releasedomain.ReleaseSucceeded)
	if err := observer.syncWorkload(context.Background(), spec, "demo-api", "devflow", []appsv1.Deployment{deployment}, nil); err != nil {
		t.Fatalf("third syncWorkload failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if got := countObserverEvents(events, "runtime_workload_state_changed"); got != 2 {
		t.Fatalf("state change event count = %d, want 2; events=%v", got, events)
	}
}

func TestReleaseOwnedSelector(t *testing.T) {
	appID := uuid.New()
	selector, err := releaseOwnedSelector(&runtimedomain.RuntimeSpec{ApplicationID: appID, Environment: "prod"})
	if err != nil {
		t.Fatalf("releaseOwnedSelector failed: %v", err)
	}
	wantParts := []string{
		releasedomain.ReleaseApplicationLabel + "=" + appID.String(),
		releasedomain.ReleaseEnvironmentLabel + "=prod",
	}
	for _, part := range wantParts {
		if !strings.Contains(selector, part) {
			t.Fatalf("selector %q missing %q", selector, part)
		}
	}
	if strings.Contains(selector, "otel.devflow.io/") {
		t.Fatalf("selector must stay label-only, got %q", selector)
	}
	if _, err := releaseOwnedSelector(&runtimedomain.RuntimeSpec{ApplicationID: appID}); err == nil {
		t.Fatal("expected environment-missing selector error")
	}
}

func TestLabelsMatchRuntimeSpec(t *testing.T) {
	appID := uuid.New()
	releaseID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{ApplicationID: appID, Environment: "prod"}

	if !labelsMatchRuntimeSpec(spec, map[string]string{
		releasedomain.ReleaseApplicationLabel: appID.String(),
		releasedomain.ReleaseEnvironmentLabel: "prod",
		releasedomain.ReleaseIDLabel:          releaseID.String(),
	}) {
		t.Fatal("expected release-owned labels to match runtime spec")
	}

	if labelsMatchRuntimeSpec(spec, map[string]string{
		releasedomain.ReleaseApplicationLabel: appID.String(),
		releasedomain.ReleaseEnvironmentLabel: "prod",
	}) {
		t.Fatal("expected missing release id label to fail match")
	}
}

func TestLabelsMatchRuntimeSpecIgnoresAnnotationStyleMetadata(t *testing.T) {
	appID := uuid.New()
	releaseID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{ApplicationID: appID, Environment: "prod"}
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo-api",
			Labels: map[string]string{
				releasedomain.ReleaseApplicationLabel: appID.String(),
				releasedomain.ReleaseEnvironmentLabel: "prod",
			},
			Annotations: map[string]string{
				"otel.devflow.io/release-id": releaseID.String(),
			},
		},
	}

	if deploymentMatchesRuntimeSpec(spec, deployment) {
		t.Fatalf("expected deployment correlation to reject annotation-only release metadata: labels=%#v annotations=%#v", deployment.Labels, deployment.Annotations)
	}
}

func TestRolloutMatchesRuntimeSpec(t *testing.T) {
	appID := uuid.New()
	releaseID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{ApplicationID: appID, Environment: "prod"}
	rollout := unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": "demo-api",
			"labels": map[string]any{
				releasedomain.ReleaseApplicationLabel: appID.String(),
				releasedomain.ReleaseEnvironmentLabel: "prod",
				releasedomain.ReleaseIDLabel:          releaseID.String(),
			},
		},
	}}

	if !rolloutMatchesRuntimeSpec(spec, rollout) {
		t.Fatal("expected release-owned rollout labels to match runtime spec")
	}
}

func TestSelectReleaseOwnedRolloutPrefersNamedMatch(t *testing.T) {
	appID := uuid.New()
	spec := &runtimedomain.RuntimeSpec{ApplicationID: appID, Environment: "prod"}
	items := []unstructured.Unstructured{
		newTestRollout("demo-api-canary", appID, "prod"),
		newTestRollout("demo-api", appID, "prod"),
	}
	picked, err := selectReleaseOwnedRollout(spec, "demo-api", items)
	if err != nil {
		t.Fatalf("selectReleaseOwnedRollout failed: %v", err)
	}
	if picked == nil || picked.GetName() != "demo-api" {
		t.Fatalf("picked = %#v", picked)
	}
}

func TestSummarizeRolloutStatus(t *testing.T) {
	healthy := newTestRollout("demo-api", uuid.New(), "prod")
	healthy.Object["status"] = map[string]any{
		"phase":         "Healthy",
		"readyReplicas": int64(2),
	}
	healthy.Object["spec"] = map[string]any{"replicas": int64(2)}
	if got := summarizeRolloutStatus(&healthy); got != "Healthy" {
		t.Fatalf("healthy status = %q", got)
	}

	progressing := newTestRollout("demo-api", uuid.New(), "prod")
	progressing.Object["status"] = map[string]any{
		"phase":         "Progressing",
		"readyReplicas": int64(1),
	}
	progressing.Object["spec"] = map[string]any{"replicas": int64(2)}
	if got := summarizeRolloutStatus(&progressing); got != "Progressing" {
		t.Fatalf("progressing status = %q", got)
	}

	degraded := newTestRollout("demo-api", uuid.New(), "prod")
	degraded.Object["status"] = map[string]any{"phase": "Degraded"}
	if got := summarizeRolloutStatus(&degraded); got != "Degraded" {
		t.Fatalf("degraded status = %q", got)
	}
}

func TestListDeploymentsFromCacheOrAPIUsesCacheWhenReady(t *testing.T) {
	appID := uuid.New()
	selector, err := labels.Parse(strings.Join([]string{
		releasedomain.ReleaseApplicationLabel + "=" + appID.String(),
		releasedomain.ReleaseEnvironmentLabel + "=prod",
	}, ","))
	if err != nil {
		t.Fatalf("labels.Parse failed: %v", err)
	}

	cache := stubWorkloadCache{
		ready: true,
		deployments: []appsv1.Deployment{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-api",
				Namespace: "default",
				Labels: map[string]string{
					releasedomain.ReleaseApplicationLabel: appID.String(),
					releasedomain.ReleaseEnvironmentLabel: "prod",
					releasedomain.ReleaseIDLabel:          uuid.New().String(),
				},
			},
		}},
	}
	observer := &KubernetesRuntimeObserver{
		workloadCache: cache,
	}

	items, err := observer.listDeploymentsFromCacheOrAPI(context.Background(), "default", selector)
	if err != nil {
		t.Fatalf("listDeploymentsFromCacheOrAPI failed: %v", err)
	}
	if len(items) != 1 || items[0].Name != "demo-api" {
		t.Fatalf("items = %#v", items)
	}
}

func TestListPodsFromCacheOrAPIUsesCacheWhenReady(t *testing.T) {
	cache := stubWorkloadCache{
		ready: true,
		pods: []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod",
				Namespace: "default",
			},
		}},
	}
	observer := &KubernetesRuntimeObserver{
		workloadCache: cache,
	}

	items, err := observer.listPodsFromCacheOrAPI(context.Background(), "default", labels.Everything())
	if err != nil {
		t.Fatalf("listPodsFromCacheOrAPI failed: %v", err)
	}
	if len(items) != 1 || items[0].Name != "demo-pod" {
		t.Fatalf("items = %#v", items)
	}
}

func TestResolveSpecNamespaceDoesNotFallBackToObserverPodNamespace(t *testing.T) {
	t.Setenv("POD_NAMESPACE", "devflow")

	observer := &KubernetesRuntimeObserver{
		cfg:   KubernetesRuntimeObserverConfig{},
		store: runtimerepo.NewMemoryStore(),
	}

	spec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: uuid.New(),
		Environment:   "ce3e0499-e862-4322-98e2-264fa6f09286",
	}

	if got := observer.resolveSpecNamespace(spec); got != "" {
		t.Fatalf("resolveSpecNamespace() = %q, want empty for cluster-wide lookup", got)
	}
}

func TestReleaseStatusLabelUpdaterUpdatesDeploymentLabels(t *testing.T) {
	clientset := kubefake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-api",
			Namespace: "devflow",
			Labels: map[string]string{
				releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
					},
				},
			},
		},
	})
	updater := NewReleaseStatusLabelUpdater(clientset, nil)

	err := updater.UpdateReleaseStatusLabel(context.Background(), &runtimedomain.RuntimeObservedWorkload{
		Namespace:    "devflow",
		WorkloadKind: "Deployment",
		WorkloadName: "demo-api",
	}, releasedomain.ReleaseSucceeded)
	if err != nil {
		t.Fatalf("UpdateReleaseStatusLabel failed: %v", err)
	}

	item, err := clientset.AppsV1().Deployments("devflow").Get(context.Background(), "demo-api", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got := item.Labels[releasedomain.ReleaseStatusLabel]; got != string(releasedomain.ReleaseSucceeded) {
		t.Fatalf("deployment label = %q", got)
	}
	if got := item.Spec.Template.Labels[releasedomain.ReleaseStatusLabel]; got != string(releasedomain.ReleaseSucceeded) {
		t.Fatalf("template label = %q", got)
	}
}

func TestReleaseStatusLabelUpdaterUpdatesRolloutLabels(t *testing.T) {
	scheme := runtime.NewScheme()
	rollout := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Rollout",
		"metadata": map[string]any{
			"name":      "demo-api",
			"namespace": "devflow",
			"labels": map[string]any{
				releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
			},
		},
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{
						releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
					},
				},
			},
		},
	}}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, rollout)
	updater := NewReleaseStatusLabelUpdater(nil, dynamicClient)

	err := updater.UpdateReleaseStatusLabel(context.Background(), &runtimedomain.RuntimeObservedWorkload{
		Namespace:    "devflow",
		WorkloadKind: "Rollout",
		WorkloadName: "demo-api",
	}, releasedomain.ReleaseSucceeded)
	if err != nil {
		t.Fatalf("UpdateReleaseStatusLabel failed: %v", err)
	}

	item, err := dynamicClient.Resource(releaseRolloutGVR).Namespace("devflow").Get(context.Background(), "demo-api", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got := item.GetLabels()[releasedomain.ReleaseStatusLabel]; got != string(releasedomain.ReleaseSucceeded) {
		t.Fatalf("rollout label = %q", got)
	}
	templateLabels, found, err := unstructured.NestedStringMap(item.Object, "spec", "template", "metadata", "labels")
	if err != nil {
		t.Fatalf("NestedStringMap failed: %v", err)
	}
	if !found {
		t.Fatal("expected rollout template labels")
	}
	if got := templateLabels[releasedomain.ReleaseStatusLabel]; got != string(releasedomain.ReleaseSucceeded) {
		t.Fatalf("rollout template label = %q", got)
	}
}

type stubWorkloadCache struct {
	ready       bool
	deployments []appsv1.Deployment
	rollouts    []unstructured.Unstructured
	pods        []corev1.Pod
}

func (s stubWorkloadCache) Start(context.Context) error                      { return nil }
func (s stubWorkloadCache) Ready() bool                                      { return s.ready }
func (s stubWorkloadCache) AddEventHandler(cache.ResourceEventHandler) error { return nil }
func (s stubWorkloadCache) ListDeployments(namespace string, selector labels.Selector) ([]appsv1.Deployment, error) {
	return s.deployments, nil
}
func (s stubWorkloadCache) ListRollouts(namespace string, selector labels.Selector) ([]unstructured.Unstructured, error) {
	return s.rollouts, nil
}
func (s stubWorkloadCache) ListPods(namespace string, selector labels.Selector) ([]corev1.Pod, error) {
	return s.pods, nil
}

func newTestRollout(name string, appID uuid.UUID, environment string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Rollout",
		"metadata": map[string]any{
			"name": name,
			"labels": map[string]any{
				releasedomain.ReleaseApplicationLabel: appID.String(),
				releasedomain.ReleaseEnvironmentLabel: environment,
				releasedomain.ReleaseIDLabel:          uuid.New().String(),
			},
		},
	}}
}

func int32PtrForObserverTest(value int32) *int32 {
	return &value
}

func countObserverEvents(events []string, target string) int {
	count := 0
	for _, event := range events {
		if event == target {
			count++
		}
	}
	return count
}
