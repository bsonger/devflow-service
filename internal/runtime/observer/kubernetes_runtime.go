package observer

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/bsonger/devflow-service/internal/runtime/repository"
	runtimeservice "github.com/bsonger/devflow-service/internal/runtime/service"
	runtimewatch "github.com/bsonger/devflow-service/internal/runtime/watch"
	"github.com/google/uuid"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type KubernetesRuntimeObserverConfig struct {
	Enabled        bool
	Namespace      string
	PollInterval   time.Duration
	ControlPlaneID string
}

type KubernetesRuntimeObserver struct {
	cfg           KubernetesRuntimeObserverConfig
	clientset     kubernetes.Interface
	dynamic       dynamic.Interface
	store         repository.Store
	runtime       runtimeservice.Service
	workloadCache runtimewatch.WorkloadCache
}

type ReleaseStatusLabelUpdater struct {
	clientset kubernetes.Interface
	dynamic   dynamic.Interface
}

func NewReleaseStatusLabelUpdater(clientset kubernetes.Interface, dynamicClient dynamic.Interface) *ReleaseStatusLabelUpdater {
	return &ReleaseStatusLabelUpdater{
		clientset: clientset,
		dynamic:   dynamicClient,
	}
}

func (u *ReleaseStatusLabelUpdater) UpdateReleaseStatusLabel(ctx context.Context, workload *domain.RuntimeObservedWorkload, status releasedomain.ReleaseStatus) error {
	if u == nil || workload == nil {
		return nil
	}
	namespace := strings.TrimSpace(workload.Namespace)
	name := strings.TrimSpace(workload.WorkloadName)
	if namespace == "" || name == "" {
		return nil
	}

	switch strings.TrimSpace(workload.WorkloadKind) {
	case "Deployment":
		item, err := u.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		labels := item.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[releasedomain.ReleaseStatusLabel] = string(status)
		item.SetLabels(labels)
		if item.Spec.Template.Labels == nil {
			item.Spec.Template.Labels = map[string]string{}
		}
		item.Spec.Template.Labels[releasedomain.ReleaseStatusLabel] = string(status)
		_, err = u.clientset.AppsV1().Deployments(namespace).Update(ctx, item, metav1.UpdateOptions{})
		return err
	case "Rollout":
		item, err := u.dynamic.Resource(releaseRolloutGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		labels := item.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[releasedomain.ReleaseStatusLabel] = string(status)
		item.SetLabels(labels)
		templateLabels, found, err := unstructured.NestedStringMap(item.Object, "spec", "template", "metadata", "labels")
		if err != nil {
			return err
		}
		if !found || templateLabels == nil {
			templateLabels = map[string]string{}
		}
		templateLabels[releasedomain.ReleaseStatusLabel] = string(status)
		if err := unstructured.SetNestedStringMap(item.Object, templateLabels, "spec", "template", "metadata", "labels"); err != nil {
			return err
		}
		_, err = u.dynamic.Resource(releaseRolloutGVR).Namespace(namespace).Update(ctx, item, metav1.UpdateOptions{})
		return err
	default:
		return nil
	}
}

func StartKubernetesRuntimeObserver(ctx context.Context, restCfg *rest.Config, cfg KubernetesRuntimeObserverConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultObserverInterval
	}
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return err
	}
	dynamicClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return err
	}
	workloadCache, err := runtimewatch.NewWorkloadCache(restCfg, runtimewatch.WorkloadCacheConfig{
		Namespace:    cfg.Namespace,
		ResyncPeriod: 10 * time.Minute,
	})
	if err != nil {
		return err
	}
	if err := workloadCache.Start(ctx); err != nil {
		return err
	}
	store := repository.RuntimeStore
	observer := &KubernetesRuntimeObserver{
		cfg:           cfg,
		clientset:     clientset,
		dynamic:       dynamicClient,
		store:         store,
		runtime:       runtimeservice.New(store, nil),
		workloadCache: workloadCache,
	}
	go observer.run(ctx)
	return nil
}

func (o *KubernetesRuntimeObserver) run(ctx context.Context) {
	ticker := time.NewTicker(o.cfg.PollInterval)
	defer ticker.Stop()
	o.sync(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.sync(ctx)
		}
	}
}

func (o *KubernetesRuntimeObserver) sync(ctx context.Context) {
	start := time.Now()
	success := false
	defer func() { observeRuntimeObserverSync(ctx, "kubernetes_runtime", success, time.Since(start)) }()
	log := platformobs.OperationLogger(ctx, "runtime_observer", "sync_kubernetes_runtime", "runtime")
	namespace := strings.TrimSpace(o.cfg.Namespace)
	if namespace == "" {
		namespace = detectObserverNamespace()
	}
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	selector := releaseDiscoverySelector(strings.TrimSpace(o.cfg.ControlPlaneID))
	var deployments *appsv1.DeploymentList
	err := platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "kubernetes",
		Operation: "list_runtime_deployments",
	}, func(depCtx context.Context) error {
		var err error
		deployments, err = o.clientset.AppsV1().Deployments(namespace).List(depCtx, metav1.ListOptions{
			LabelSelector: selector,
		})
		return err
	})
	if err != nil {
		log.Warn("list runtime deployments failed", zap.Error(err))
		return
	}
	var rollouts *unstructured.UnstructuredList
	err = platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "argo_rollouts",
		Operation: "list_runtime_rollouts",
	}, func(depCtx context.Context) error {
		var err error
		rollouts, err = o.dynamic.Resource(releaseRolloutGVR).Namespace(namespace).List(depCtx, metav1.ListOptions{
			LabelSelector: selector,
		})
		return err
	})
	if err != nil {
		log.Warn("list runtime rollouts failed", zap.Error(err))
		return
	}

	targets := make(map[uuid.UUID]runtimeObserverTarget)
	for i := range deployments.Items {
		deployment := &deployments.Items[i]
		spec, appName, ok := o.runtimeSpecFromDeployment(deployment)
		if !ok || spec == nil {
			continue
		}
		targets[spec.ID] = runtimeObserverTarget{
			spec:      spec,
			appName:   appName,
			namespace: deployment.Namespace,
		}
	}
	for i := range rollouts.Items {
		rollout := &rollouts.Items[i]
		spec, appName, ok := o.runtimeSpecFromRollout(rollout)
		if !ok || spec == nil {
			continue
		}
		targets[spec.ID] = runtimeObserverTarget{
			spec:      spec,
			appName:   appName,
			namespace: rollout.GetNamespace(),
		}
	}
	for _, target := range targets {
		if err := o.syncRuntimeSpec(ctx, target.spec, target.appName, target.namespace); err != nil {
			log.Warn("sync runtime workload from kubernetes failed",
				zap.String("runtime_spec_id", target.spec.ID.String()),
				zap.String("devflow.application.id", target.spec.ApplicationID.String()),
				zap.String("devflow.environment.id", target.spec.Environment),
				zap.String("observed_workload_namespace", target.namespace),
				zap.String("observed_workload_name", target.appName),
				zap.Error(err),
			)
		}
	}
	success = true
}

type runtimeObserverTarget struct {
	spec      *domain.RuntimeSpec
	appName   string
	namespace string
}

func (o *KubernetesRuntimeObserver) syncRuntimeSpec(ctx context.Context, spec *domain.RuntimeSpec, appName, targetNamespace string) error {
	targetNamespace = strings.TrimSpace(targetNamespace)
	if targetNamespace == "" {
		targetNamespace = o.resolveSpecNamespace(spec)
	}
	selector, err := releaseOwnedSelectorWithControlPlane(spec, strings.TrimSpace(o.cfg.ControlPlaneID))
	if err != nil {
		return err
	}
	parsedSelector, err := labels.Parse(selector)
	if err != nil {
		return err
	}
	deployments, err := o.listDeploymentsFromCacheOrAPI(ctx, targetNamespace, parsedSelector)
	if err != nil {
		return err
	}
	rollouts, err := o.listRolloutsFromCacheOrAPI(ctx, targetNamespace, parsedSelector)
	if err != nil {
		return err
	}
	if err := o.syncWorkload(ctx, spec, appName, targetNamespace, deployments, rollouts); err != nil {
		return err
	}
	pods, err := o.listPodsFromCacheOrAPI(ctx, targetNamespace, parsedSelector)
	if err != nil {
		return err
	}
	return o.syncPods(ctx, spec, targetNamespace, pods)
}

func (o *KubernetesRuntimeObserver) runtimeSpecFromDeployment(deployment *appsv1.Deployment) (*domain.RuntimeSpec, string, bool) {
	if deployment == nil {
		return nil, "", false
	}
	labels := deployment.GetLabels()
	applicationID, err := uuid.Parse(strings.TrimSpace(labels[releasedomain.ReleaseApplicationLabel]))
	if err != nil || applicationID == uuid.Nil {
		return nil, "", false
	}
	environment := strings.TrimSpace(labels[releasedomain.ReleaseEnvironmentLabel])
	if environment == "" {
		return nil, "", false
	}
	appName := strings.TrimSpace(labels["app.kubernetes.io/name"])
	if appName == "" {
		appName = strings.TrimSpace(deployment.Name)
	}
	if appName == "" {
		return nil, "", false
	}
	spec, err := o.store.EnsureRuntimeSpecByApplicationEnv(context.Background(), applicationID, environment)
	if err != nil || spec == nil {
		return nil, "", false
	}
	return spec, appName, true
}

func (o *KubernetesRuntimeObserver) runtimeSpecFromRollout(rollout *unstructured.Unstructured) (*domain.RuntimeSpec, string, bool) {
	if rollout == nil {
		return nil, "", false
	}
	labels := rollout.GetLabels()
	applicationID, err := uuid.Parse(strings.TrimSpace(labels[releasedomain.ReleaseApplicationLabel]))
	if err != nil || applicationID == uuid.Nil {
		return nil, "", false
	}
	environment := strings.TrimSpace(labels[releasedomain.ReleaseEnvironmentLabel])
	if environment == "" {
		return nil, "", false
	}
	appName := strings.TrimSpace(labels["app.kubernetes.io/name"])
	if appName == "" {
		appName = strings.TrimSpace(rollout.GetName())
	}
	if appName == "" {
		return nil, "", false
	}
	spec, err := o.store.EnsureRuntimeSpecByApplicationEnv(context.Background(), applicationID, environment)
	if err != nil || spec == nil {
		return nil, "", false
	}
	return spec, appName, true
}

func (o *KubernetesRuntimeObserver) syncWorkload(ctx context.Context, spec *domain.RuntimeSpec, appName, namespace string, deployments []appsv1.Deployment, rollouts []unstructured.Unstructured) error {
	rollout, err := selectReleaseOwnedRollout(spec, appName, rollouts)
	if err != nil {
		return err
	}
	if rollout != nil {
		return o.syncRollout(ctx, spec, namespace, rollout)
	}

	deployment, err := selectReleaseOwnedDeployment(spec, appName, deployments)
	if err != nil {
		return err
	}

	if deployment == nil {
		existing, err := o.store.GetObservedWorkload(ctx, spec.ID)
		if err == nil && existing != nil {
			return o.runtime.DeleteObservedWorkload(ctx, runtimeservice.DeleteObservedWorkloadInput{
				ApplicationID: spec.ApplicationID,
				Environment:   spec.Environment,
				Namespace:     existing.Namespace,
				WorkloadKind:  existing.WorkloadKind,
				WorkloadName:  existing.WorkloadName,
				ObservedAt:    time.Now().UTC(),
			})
		}
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	restartAt := parseRestartAt(deployment.Spec.Template.Annotations)
	_, err = o.runtime.SyncObservedWorkload(ctx, runtimeservice.SyncObservedWorkloadInput{
		ApplicationID:       spec.ApplicationID,
		Environment:         spec.Environment,
		Namespace:           namespace,
		WorkloadKind:        "Deployment",
		WorkloadName:        deployment.Name,
		DesiredReplicas:     int32Value(deployment.Spec.Replicas),
		ReadyReplicas:       int(deployment.Status.ReadyReplicas),
		UpdatedReplicas:     int(deployment.Status.UpdatedReplicas),
		AvailableReplicas:   int(deployment.Status.AvailableReplicas),
		UnavailableReplicas: int(deployment.Status.UnavailableReplicas),
		ObservedGeneration:  deployment.Status.ObservedGeneration,
		SummaryStatus:       summarizeDeploymentStatus(*deployment),
		Images:              deploymentImages(*deployment),
		Conditions:          deploymentConditions(deployment.Status.Conditions),
		Labels:              deployment.Labels,
		Annotations:         deployment.Spec.Template.Annotations,
		ObservedAt:          time.Now().UTC(),
		RestartAt:           restartAt,
	})
	return err
}

func (o *KubernetesRuntimeObserver) syncRollout(ctx context.Context, spec *domain.RuntimeSpec, namespace string, rollout *unstructured.Unstructured) error {
	if rollout == nil {
		return nil
	}
	restartAt := parseRestartAt(rolloutTemplateAnnotations(rollout))
	_, err := o.runtime.SyncObservedWorkload(ctx, runtimeservice.SyncObservedWorkloadInput{
		ApplicationID:       spec.ApplicationID,
		Environment:         spec.Environment,
		Namespace:           namespace,
		WorkloadKind:        "Rollout",
		WorkloadName:        rollout.GetName(),
		DesiredReplicas:     int(nestedInt64Default(rollout.Object, 0, "spec", "replicas")),
		ReadyReplicas:       int(nestedInt64Default(rollout.Object, 0, "status", "readyReplicas")),
		UpdatedReplicas:     int(nestedInt64Default(rollout.Object, 0, "status", "updatedReplicas")),
		AvailableReplicas:   int(nestedInt64Default(rollout.Object, 0, "status", "availableReplicas")),
		UnavailableReplicas: int(nestedInt64Default(rollout.Object, 0, "status", "unavailableReplicas")),
		ObservedGeneration:  nestedInt64Default(rollout.Object, 0, "status", "observedGeneration"),
		SummaryStatus:       summarizeRolloutStatus(rollout),
		Images:              rolloutImages(rollout),
		Conditions:          rolloutConditions(rollout),
		Labels:              rollout.GetLabels(),
		Annotations:         rolloutTemplateAnnotations(rollout),
		ObservedAt:          time.Now().UTC(),
		RestartAt:           restartAt,
	})
	return err
}

func (o *KubernetesRuntimeObserver) syncPods(ctx context.Context, spec *domain.RuntimeSpec, namespace string, pods []corev1.Pod) error {
	existing, err := o.store.ListObservedPods(ctx, spec.ID)
	if err != nil {
		return err
	}
	existingByName := make(map[string]*domain.RuntimeObservedPod, len(existing))
	for _, item := range existing {
		if item == nil {
			continue
		}
		existingByName[item.PodName] = item
	}
	now := time.Now().UTC()
	seenNames := make(map[string]struct{}, len(pods))
	for _, pod := range filterReleaseOwnedPods(spec, pods) {
		seenNames[pod.Name] = struct{}{}
		_, err := o.runtime.SyncObservedPod(ctx, runtimeservice.SyncObservedPodInput{
			ApplicationID: spec.ApplicationID,
			Environment:   spec.Environment,
			Namespace:     namespace,
			PodName:       pod.Name,
			Phase:         string(pod.Status.Phase),
			Ready:         isPodReady(pod),
			Restarts:      podRestarts(pod.Status.ContainerStatuses),
			NodeName:      strings.TrimSpace(pod.Spec.NodeName),
			PodIP:         strings.TrimSpace(pod.Status.PodIP),
			HostIP:        strings.TrimSpace(pod.Status.HostIP),
			OwnerKind:     ownerKind(pod.OwnerReferences),
			OwnerName:     ownerName(pod.OwnerReferences),
			Labels:        pod.Labels,
			Containers:    podContainers(pod.Status.ContainerStatuses),
			ObservedAt:    now,
		})
		if err != nil {
			return err
		}
	}
	for podName, item := range existingByName {
		if item == nil {
			continue
		}
		if item != nil && item.Namespace == namespace {
			if _, ok := seenNames[podName]; ok {
				continue
			}
		}
		if err := o.store.DeleteObservedPod(ctx, spec.ID, item.Namespace, podName, now); err != nil {
			return err
		}
	}
	return nil
}

func releaseOwnedSelector(spec *domain.RuntimeSpec) (string, error) {
	return releaseOwnedSelectorWithControlPlane(spec, "")
}

func releaseOwnedSelectorWithControlPlane(spec *domain.RuntimeSpec, controlPlaneID string) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("runtime spec is required for release-owned correlation")
	}
	if spec.ApplicationID == uuid.Nil {
		return "", fmt.Errorf("runtime spec application id is required for release-owned correlation")
	}
	environment := strings.TrimSpace(spec.Environment)
	if environment == "" {
		return "", fmt.Errorf("runtime spec environment is required for release-owned correlation")
	}
	parts := []string{
		releasedomain.ReleaseApplicationLabel + "=" + spec.ApplicationID.String(),
		releasedomain.ReleaseEnvironmentLabel + "=" + environment,
	}
	if strings.TrimSpace(controlPlaneID) != "" {
		parts = append(parts, releasedomain.ControlPlaneLabel+"="+strings.TrimSpace(controlPlaneID))
	}
	return strings.Join(parts, ","), nil
}

func deploymentMatchesRuntimeSpec(spec *domain.RuntimeSpec, deployment appsv1.Deployment) bool {
	return labelsMatchRuntimeSpec(spec, deployment.GetLabels())
}

func podMatchesRuntimeSpec(spec *domain.RuntimeSpec, pod corev1.Pod) bool {
	return labelsMatchRuntimeSpec(spec, pod.GetLabels())
}

func (o *KubernetesRuntimeObserver) listDeploymentsFromCacheOrAPI(ctx context.Context, namespace string, selector labels.Selector) ([]appsv1.Deployment, error) {
	if o.workloadCache != nil && o.workloadCache.Ready() {
		items, err := o.workloadCache.ListDeployments(namespace, selector)
		if err == nil {
			return items, nil
		}
	}

	var result *appsv1.DeploymentList
	err := platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "kubernetes",
		Operation: "list_release_owned_deployments",
	}, func(depCtx context.Context) error {
		var err error
		result, err = o.clientset.AppsV1().Deployments(namespace).List(depCtx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (o *KubernetesRuntimeObserver) listRolloutsFromCacheOrAPI(ctx context.Context, namespace string, selector labels.Selector) ([]unstructured.Unstructured, error) {
	if o.workloadCache != nil && o.workloadCache.Ready() {
		items, err := o.workloadCache.ListRollouts(namespace, selector)
		if err == nil {
			return items, nil
		}
	}

	var result *unstructured.UnstructuredList
	err := platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "argo_rollouts",
		Operation: "list_release_owned_rollouts",
	}, func(depCtx context.Context) error {
		var err error
		result, err = o.dynamic.Resource(releaseRolloutGVR).Namespace(namespace).List(depCtx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (o *KubernetesRuntimeObserver) listPodsFromCacheOrAPI(ctx context.Context, namespace string, selector labels.Selector) ([]corev1.Pod, error) {
	if o.workloadCache != nil && o.workloadCache.Ready() {
		items, err := o.workloadCache.ListPods(namespace, selector)
		if err == nil {
			return items, nil
		}
	}

	var result *corev1.PodList
	err := platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "kubernetes",
		Operation: "list_release_owned_pods",
	}, func(depCtx context.Context) error {
		var err error
		result, err = o.clientset.CoreV1().Pods(namespace).List(depCtx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func labelsMatchRuntimeSpec(spec *domain.RuntimeSpec, labels map[string]string) bool {
	return labelsMatchRuntimeSpecWithControlPlane(spec, labels, "")
}

func labelsMatchRuntimeSpecWithControlPlane(spec *domain.RuntimeSpec, labels map[string]string, controlPlaneID string) bool {
	if spec == nil {
		return false
	}
	if spec.ApplicationID == uuid.Nil {
		return false
	}
	if strings.TrimSpace(spec.Environment) == "" {
		return false
	}
	if len(labels) == 0 {
		return false
	}
	if strings.TrimSpace(labels[releasedomain.ReleaseApplicationLabel]) != spec.ApplicationID.String() {
		return false
	}
	if strings.TrimSpace(labels[releasedomain.ReleaseEnvironmentLabel]) != strings.TrimSpace(spec.Environment) {
		return false
	}
	if strings.TrimSpace(labels[releasedomain.ReleaseIDLabel]) == "" {
		return false
	}
	if strings.TrimSpace(controlPlaneID) != "" && strings.TrimSpace(labels[releasedomain.ControlPlaneLabel]) != strings.TrimSpace(controlPlaneID) {
		return false
	}
	return true
}

func selectReleaseOwnedDeployment(spec *domain.RuntimeSpec, appName string, deployments []appsv1.Deployment) (*appsv1.Deployment, error) {
	matches := make([]appsv1.Deployment, 0, len(deployments))
	for _, deployment := range deployments {
		if !deploymentMatchesRuntimeSpec(spec, deployment) {
			continue
		}
		matches = append(matches, deployment)
	}
	if len(matches) == 0 {
		return nil, nil
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	preferred := make([]appsv1.Deployment, 0, len(matches))
	for _, deployment := range matches {
		if strings.TrimSpace(appName) != "" && deployment.Name == strings.TrimSpace(appName) {
			preferred = append(preferred, deployment)
		}
	}
	if len(preferred) == 1 {
		return &preferred[0], nil
	}
	if len(preferred) > 1 {
		matches = preferred
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].Name < matches[j].Name
	})
	return nil, fmt.Errorf("%w: application_id=%s environment=%s namespace=%s candidates=%s", runtimeservice.ErrRuntimeWorkloadAmbiguous, spec.ApplicationID.String(), strings.TrimSpace(spec.Environment), strings.TrimSpace(matches[0].Namespace), joinDeploymentNames(matches))
}

func rolloutMatchesRuntimeSpec(spec *domain.RuntimeSpec, rollout unstructured.Unstructured) bool {
	return labelsMatchRuntimeSpec(spec, rollout.GetLabels())
}

func releaseDiscoverySelector(controlPlaneID string) string {
	parts := []string{releasedomain.ReleaseApplicationLabel}
	if strings.TrimSpace(controlPlaneID) != "" {
		parts = append(parts, releasedomain.ControlPlaneLabel+"="+strings.TrimSpace(controlPlaneID))
	}
	return strings.Join(parts, ",")
}

func selectReleaseOwnedRollout(spec *domain.RuntimeSpec, appName string, rollouts []unstructured.Unstructured) (*unstructured.Unstructured, error) {
	matches := make([]unstructured.Unstructured, 0, len(rollouts))
	for _, rollout := range rollouts {
		if !rolloutMatchesRuntimeSpec(spec, rollout) {
			continue
		}
		matches = append(matches, rollout)
	}
	if len(matches) == 0 {
		return nil, nil
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	preferred := make([]unstructured.Unstructured, 0, len(matches))
	for _, rollout := range matches {
		if strings.TrimSpace(appName) != "" && rollout.GetName() == strings.TrimSpace(appName) {
			preferred = append(preferred, rollout)
		}
	}
	if len(preferred) == 1 {
		return &preferred[0], nil
	}
	if len(preferred) > 1 {
		matches = preferred
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].GetName() < matches[j].GetName()
	})
	return nil, fmt.Errorf("%w: application_id=%s environment=%s namespace=%s rollout_candidates=%s", runtimeservice.ErrRuntimeWorkloadAmbiguous, spec.ApplicationID.String(), strings.TrimSpace(spec.Environment), strings.TrimSpace(matches[0].GetNamespace()), joinRolloutNames(matches))
}

func filterReleaseOwnedPods(spec *domain.RuntimeSpec, pods []corev1.Pod) []corev1.Pod {
	if len(pods) == 0 {
		return nil
	}
	filtered := make([]corev1.Pod, 0, len(pods))
	for _, pod := range pods {
		if !podMatchesRuntimeSpec(spec, pod) {
			continue
		}
		filtered = append(filtered, pod)
	}
	return filtered
}

func joinDeploymentNames(items []appsv1.Deployment) string {
	if len(items) == 0 {
		return ""
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func joinRolloutNames(items []unstructured.Unstructured) string {
	if len(items) == 0 {
		return ""
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.GetName())
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func (o *KubernetesRuntimeObserver) resolveSpecNamespace(spec *domain.RuntimeSpec) string {
	if spec == nil {
		return strings.TrimSpace(o.cfg.Namespace)
	}
	if workload, err := o.store.GetObservedWorkload(context.Background(), spec.ID); err == nil && workload != nil {
		if namespace := strings.TrimSpace(workload.Namespace); namespace != "" {
			return namespace
		}
	}
	namespace := strings.TrimSpace(o.cfg.Namespace)
	if namespace == "" {
		namespace = detectObserverNamespace()
	}
	return namespace
}

func detectObserverNamespace() string {
	if ns := strings.TrimSpace(os.Getenv("POD_NAMESPACE")); ns != "" {
		return ns
	}
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); ns != "" {
			return ns
		}
	}
	return ""
}

func int32Value(v *int32) int {
	if v == nil {
		return 0
	}
	return int(*v)
}

func summarizeDeploymentStatus(deployment appsv1.Deployment) string {
	for _, cond := range deployment.Status.Conditions {
		if cond.Type == appsv1.DeploymentProgressing && cond.Status == corev1.ConditionFalse {
			return "Degraded"
		}
	}
	if deployment.Status.UnavailableReplicas > 0 {
		return "Progressing"
	}
	if deployment.Status.ReadyReplicas == deployment.Status.Replicas && deployment.Status.Replicas > 0 {
		return "Healthy"
	}
	if deployment.Status.Replicas == 0 {
		return "Idle"
	}
	return "Unknown"
}

func summarizeRolloutStatus(rollout *unstructured.Unstructured) string {
	if rollout == nil {
		return "Unknown"
	}
	phase := strings.ToLower(strings.TrimSpace(nestedString(rollout.Object, "status", "phase")))
	switch phase {
	case "healthy", "completed":
		return "Healthy"
	case "degraded", "error", "failed":
		return "Degraded"
	case "paused", "progressing":
		return "Progressing"
	}
	desired := nestedInt64Default(rollout.Object, 0, "spec", "replicas")
	ready := nestedInt64Default(rollout.Object, 0, "status", "readyReplicas")
	if desired == 0 {
		return "Idle"
	}
	if ready >= desired {
		return "Healthy"
	}
	if ready > 0 {
		return "Progressing"
	}
	return "Unknown"
}

func deploymentImages(deployment appsv1.Deployment) []string {
	images := make([]string, 0, len(deployment.Spec.Template.Spec.Containers))
	for _, c := range deployment.Spec.Template.Spec.Containers {
		image := strings.TrimSpace(c.Image)
		if image == "" {
			continue
		}
		images = append(images, image)
	}
	return images
}

func rolloutImages(rollout *unstructured.Unstructured) []string {
	if rollout == nil {
		return nil
	}
	containers, _, _ := unstructured.NestedSlice(rollout.Object, "spec", "template", "spec", "containers")
	images := make([]string, 0, len(containers))
	for _, item := range containers {
		container, ok := item.(map[string]any)
		if !ok {
			continue
		}
		image := strings.TrimSpace(fmt.Sprintf("%v", container["image"]))
		if image == "" {
			continue
		}
		images = append(images, image)
	}
	return images
}

func deploymentConditions(conditions []appsv1.DeploymentCondition) []runtimeservice.ObservedWorkloadConditionInput {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]runtimeservice.ObservedWorkloadConditionInput, 0, len(conditions))
	for _, cond := range conditions {
		ts := cond.LastTransitionTime.UTC()
		out = append(out, runtimeservice.ObservedWorkloadConditionInput{
			Type:               string(cond.Type),
			Status:             string(cond.Status),
			Reason:             strings.TrimSpace(cond.Reason),
			Message:            strings.TrimSpace(cond.Message),
			LastTransitionTime: &ts,
		})
	}
	return out
}

func rolloutConditions(rollout *unstructured.Unstructured) []runtimeservice.ObservedWorkloadConditionInput {
	if rollout == nil {
		return nil
	}
	items, _, _ := unstructured.NestedSlice(rollout.Object, "status", "conditions")
	if len(items) == 0 {
		return nil
	}
	out := make([]runtimeservice.ObservedWorkloadConditionInput, 0, len(items))
	for _, item := range items {
		condition, ok := item.(map[string]any)
		if !ok {
			continue
		}
		var ts *time.Time
		if value := strings.TrimSpace(fmt.Sprintf("%v", condition["lastTransitionTime"])); value != "" {
			if parsed, err := time.Parse(time.RFC3339, value); err == nil {
				utc := parsed.UTC()
				ts = &utc
			}
		}
		out = append(out, runtimeservice.ObservedWorkloadConditionInput{
			Type:               strings.TrimSpace(fmt.Sprintf("%v", condition["type"])),
			Status:             strings.TrimSpace(fmt.Sprintf("%v", condition["status"])),
			Reason:             strings.TrimSpace(fmt.Sprintf("%v", condition["reason"])),
			Message:            strings.TrimSpace(fmt.Sprintf("%v", condition["message"])),
			LastTransitionTime: ts,
		})
	}
	return out
}

func rolloutTemplateAnnotations(rollout *unstructured.Unstructured) map[string]string {
	if rollout == nil {
		return nil
	}
	annotations, _, _ := unstructured.NestedStringMap(rollout.Object, "spec", "template", "metadata", "annotations")
	if len(annotations) == 0 {
		return nil
	}
	out := make(map[string]string, len(annotations))
	for key, value := range annotations {
		out[key] = strings.TrimSpace(value)
	}
	return out
}

func parseRestartAt(annotations map[string]string) *time.Time {
	if len(annotations) == 0 {
		return nil
	}
	value := strings.TrimSpace(annotations["kubectl.kubernetes.io/restartedAt"])
	if value == "" {
		return nil
	}
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	utc := ts.UTC()
	return &utc
}

func isPodReady(pod corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func podRestarts(statuses []corev1.ContainerStatus) int {
	total := 0
	for _, st := range statuses {
		total += int(st.RestartCount)
	}
	return total
}

func ownerKind(refs []metav1.OwnerReference) string {
	for _, ref := range refs {
		if ref.Controller != nil && *ref.Controller {
			return strings.TrimSpace(ref.Kind)
		}
	}
	return ""
}

func ownerName(refs []metav1.OwnerReference) string {
	for _, ref := range refs {
		if ref.Controller != nil && *ref.Controller {
			return strings.TrimSpace(ref.Name)
		}
	}
	return ""
}

func podContainers(statuses []corev1.ContainerStatus) []runtimeservice.ObservedPodContainerInput {
	if len(statuses) == 0 {
		return nil
	}
	out := make([]runtimeservice.ObservedPodContainerInput, 0, len(statuses))
	for _, st := range statuses {
		out = append(out, runtimeservice.ObservedPodContainerInput{
			Name:         strings.TrimSpace(st.Name),
			Image:        strings.TrimSpace(st.Image),
			ImageID:      strings.TrimSpace(st.ImageID),
			Ready:        st.Ready,
			RestartCount: int(st.RestartCount),
			State:        podContainerState(st),
		})
	}
	return out
}

func podContainerState(status corev1.ContainerStatus) string {
	switch {
	case status.State.Running != nil:
		return "Running"
	case status.State.Waiting != nil:
		return strings.TrimSpace(status.State.Waiting.Reason)
	case status.State.Terminated != nil:
		return strings.TrimSpace(status.State.Terminated.Reason)
	default:
		return ""
	}
}
