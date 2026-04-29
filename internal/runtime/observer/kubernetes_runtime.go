package observer

import (
	"context"
	"database/sql"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/bsonger/devflow-service/internal/runtime/repository"
	runtimeservice "github.com/bsonger/devflow-service/internal/runtime/service"
	"github.com/google/uuid"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type KubernetesRuntimeObserverConfig struct {
	Enabled      bool
	Namespace    string
	PollInterval time.Duration
}

type KubernetesRuntimeObserver struct {
	cfg       KubernetesRuntimeObserverConfig
	clientset kubernetes.Interface
	store     repository.Store
	runtime   runtimeservice.Service
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
	store := repository.RuntimeStore
	observer := &KubernetesRuntimeObserver{
		cfg:       cfg,
		clientset: clientset,
		store:     store,
		runtime:   runtimeservice.New(store, nil),
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
	log := logger.LoggerWithContext(ctx)
	if log == nil {
		log = zap.NewNop()
	}
	namespace := strings.TrimSpace(o.cfg.Namespace)
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	deployments, err := o.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: releasedomain.ReleaseApplicationLabel,
	})
	if err != nil {
		log.Warn("list runtime deployments failed", zap.Error(err))
		return
	}
	for i := range deployments.Items {
		deployment := &deployments.Items[i]
		spec, appName, ok := o.runtimeSpecFromDeployment(deployment)
		if !ok {
			continue
		}
		if err := o.syncRuntimeSpec(ctx, spec, appName, deployment.Namespace); err != nil {
			log.Warn("sync runtime deployment from kubernetes failed",
				zap.String("runtime_spec_id", spec.ID.String()),
				zap.String("application_id", spec.ApplicationID.String()),
				zap.String("environment", spec.Environment),
				zap.String("namespace", deployment.Namespace),
				zap.String("deployment", deployment.Name),
				zap.Error(err),
			)
		}
	}
}

func (o *KubernetesRuntimeObserver) syncRuntimeSpec(ctx context.Context, spec *domain.RuntimeSpec, appName, targetNamespace string) error {
	targetNamespace = strings.TrimSpace(targetNamespace)
	if targetNamespace == "" {
		targetNamespace = o.resolveSpecNamespace(spec)
	}
	selector := "app.kubernetes.io/name=" + strings.TrimSpace(appName)

	deployments, err := o.clientset.AppsV1().Deployments(targetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return err
	}
	if err := o.syncDeployment(ctx, spec, appName, targetNamespace, deployments.Items); err != nil {
		return err
	}

	pods, err := o.clientset.CoreV1().Pods(targetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return err
	}
	return o.syncPods(ctx, spec, targetNamespace, pods.Items)
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

func (o *KubernetesRuntimeObserver) syncDeployment(ctx context.Context, spec *domain.RuntimeSpec, appName, namespace string, deployments []appsv1.Deployment) error {
	sort.SliceStable(deployments, func(i, j int) bool {
		if deployments[i].Name == appName {
			return true
		}
		if deployments[j].Name == appName {
			return false
		}
		return deployments[i].Name < deployments[j].Name
	})

	if len(deployments) == 0 {
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

	deployment := deployments[0]
	restartAt := parseRestartAt(deployment.Spec.Template.Annotations)
	_, err := o.runtime.SyncObservedWorkload(ctx, runtimeservice.SyncObservedWorkloadInput{
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
		SummaryStatus:       summarizeDeploymentStatus(deployment),
		Images:              deploymentImages(deployment),
		Conditions:          deploymentConditions(deployment.Status.Conditions),
		Labels:              deployment.Labels,
		Annotations:         deployment.Spec.Template.Annotations,
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
	for _, pod := range pods {
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

func deploymentConditions(conditions []appsv1.DeploymentCondition) []runtimeservice.ObservedWorkloadConditionInput {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]runtimeservice.ObservedWorkloadConditionInput, 0, len(conditions))
	for _, cond := range conditions {
		ts := cond.LastTransitionTime.Time.UTC()
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
