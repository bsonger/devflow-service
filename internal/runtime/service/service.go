package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/bsonger/devflow-service/internal/runtime/repository"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var ErrDuplicateRuntimeSpec = sharederrs.Conflict("runtime spec already exists for application and environment")
var ErrRuntimeSpecNotFound = sharederrs.NotFound("runtime spec not found")
var ErrNamespaceMismatch = sharederrs.InvalidArgument("observed pod namespace does not match derived runtime namespace")
var ErrK8sClientInit = sharederrs.FailedPrecondition("kubernetes client initialization failed")
var ErrK8sNotFound = sharederrs.NotFound("kubernetes resource not found")
var ErrK8sForbidden = sharederrs.FailedPrecondition("kubernetes operation forbidden")

type K8sExecutor interface {
	DeletePod(ctx context.Context, namespace, name string) error
	RestartDeployment(ctx context.Context, namespace, name string) error
}

type Service interface {
	CreateRuntimeSpec(context.Context, CreateRuntimeSpecInput) (*domain.RuntimeSpec, error)
	ListRuntimeSpecs(context.Context) ([]*domain.RuntimeSpec, error)
	GetRuntimeSpec(context.Context, uuid.UUID) (*domain.RuntimeSpec, error)
	DeleteRuntimeSpecByApplicationEnv(context.Context, uuid.UUID, string) error
	CreateRuntimeSpecRevision(context.Context, uuid.UUID, CreateRuntimeSpecRevisionInput) (*domain.RuntimeSpecRevision, error)
	ListRuntimeSpecRevisions(context.Context, uuid.UUID) ([]*domain.RuntimeSpecRevision, error)
	GetRuntimeSpecRevision(context.Context, uuid.UUID) (*domain.RuntimeSpecRevision, error)
	GetObservedWorkload(context.Context, uuid.UUID) (*domain.RuntimeObservedWorkload, error)
	GetObservedWorkloadByApplicationEnv(context.Context, uuid.UUID, string) (*domain.RuntimeObservedWorkload, error)
	SyncObservedWorkload(context.Context, SyncObservedWorkloadInput) (*domain.RuntimeObservedWorkload, error)
	DeleteObservedWorkload(context.Context, DeleteObservedWorkloadInput) error
	ListObservedPods(context.Context, uuid.UUID) ([]*domain.RuntimeObservedPod, error)
	ListObservedPodsByApplicationEnv(context.Context, uuid.UUID, string) ([]*domain.RuntimeObservedPod, error)
	SyncObservedPod(context.Context, SyncObservedPodInput) (*domain.RuntimeObservedPod, error)
	DeleteObservedPod(context.Context, DeleteObservedPodInput) error
	DeletePod(context.Context, uuid.UUID, string, string) error
	DeletePodByApplicationEnv(context.Context, uuid.UUID, string, string, string) error
	RestartDeployment(context.Context, uuid.UUID, string, string) error
	RestartDeploymentByApplicationEnv(context.Context, uuid.UUID, string, string, string) error
	ListRuntimeOperations(context.Context, uuid.UUID) ([]*domain.RuntimeOperation, error)
}

type runtimeService struct {
	store       repository.Store
	k8sExecutor K8sExecutor
}

var DefaultService Service = New(repository.NewPostgresStore(), nil)

func New(store repository.Store, k8s K8sExecutor) Service {
	return &runtimeService{store: store, k8sExecutor: k8s}
}

type CreateRuntimeSpecInput struct {
	ApplicationID uuid.UUID `json:"application_id"`
	Environment   string    `json:"environment"`
}

type CreateRuntimeSpecRevisionInput struct {
	Replicas         int    `json:"replicas"`
	HealthThresholds string `json:"health_thresholds"`
	Resources        string `json:"resources"`
	Autoscaling      string `json:"autoscaling"`
	Scheduling       string `json:"scheduling"`
	PodEnvs          string `json:"pod_envs"`
	CreatedBy        string `json:"created_by"`
}

type ObservedWorkloadConditionInput struct {
	Type               string     `json:"type"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason,omitempty"`
	Message            string     `json:"message,omitempty"`
	LastTransitionTime *time.Time `json:"last_transition_time,omitempty"`
}

type SyncObservedWorkloadInput struct {
	ApplicationID       uuid.UUID                        `json:"application_id"`
	Environment         string                           `json:"environment"`
	Namespace           string                           `json:"namespace"`
	WorkloadKind        string                           `json:"workload_kind"`
	WorkloadName        string                           `json:"workload_name"`
	DesiredReplicas     int                              `json:"desired_replicas"`
	ReadyReplicas       int                              `json:"ready_replicas"`
	UpdatedReplicas     int                              `json:"updated_replicas"`
	AvailableReplicas   int                              `json:"available_replicas"`
	UnavailableReplicas int                              `json:"unavailable_replicas"`
	ObservedGeneration  int64                            `json:"observed_generation"`
	SummaryStatus       string                           `json:"summary_status"`
	Images              []string                         `json:"images,omitempty"`
	Conditions          []ObservedWorkloadConditionInput `json:"conditions,omitempty"`
	Labels              map[string]string                `json:"labels,omitempty"`
	Annotations         map[string]string                `json:"annotations,omitempty"`
	ObservedAt          time.Time                        `json:"observed_at"`
	RestartAt           *time.Time                       `json:"restart_at,omitempty"`
}

type DeleteObservedWorkloadInput struct {
	ApplicationID uuid.UUID `json:"application_id"`
	Environment   string    `json:"environment"`
	Namespace     string    `json:"namespace"`
	WorkloadKind  string    `json:"workload_kind"`
	WorkloadName  string    `json:"workload_name"`
	ObservedAt    time.Time `json:"observed_at"`
}

type ObservedPodContainerInput struct {
	Name         string `json:"name"`
	Image        string `json:"image,omitempty"`
	ImageID      string `json:"image_id,omitempty"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restart_count"`
	State        string `json:"state,omitempty"`
}

type SyncObservedPodInput struct {
	ApplicationID uuid.UUID                   `json:"application_id"`
	Environment   string                      `json:"environment"`
	Namespace     string                      `json:"namespace"`
	PodName       string                      `json:"pod_name"`
	Phase         string                      `json:"phase"`
	Ready         bool                        `json:"ready"`
	Restarts      int                         `json:"restarts"`
	NodeName      string                      `json:"node_name,omitempty"`
	PodIP         string                      `json:"pod_ip,omitempty"`
	HostIP        string                      `json:"host_ip,omitempty"`
	OwnerKind     string                      `json:"owner_kind,omitempty"`
	OwnerName     string                      `json:"owner_name,omitempty"`
	Labels        map[string]string           `json:"labels,omitempty"`
	Containers    []ObservedPodContainerInput `json:"containers,omitempty"`
	ObservedAt    time.Time                   `json:"observed_at"`
}

type DeleteObservedPodInput struct {
	ApplicationID uuid.UUID `json:"application_id"`
	Environment   string    `json:"environment"`
	Namespace     string    `json:"namespace"`
	PodName       string    `json:"pod_name"`
	ObservedAt    time.Time `json:"observed_at"`
}

func (s *runtimeService) repoStore() repository.Store {
	if s.store == nil {
		s.store = repository.NewPostgresStore()
	}
	return s.store
}

func (s *runtimeService) k8s() (K8sExecutor, error) {
	if s.k8sExecutor != nil {
		return s.k8sExecutor, nil
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrK8sClientInit, err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrK8sClientInit, err)
	}
	s.k8sExecutor = &k8sExecutor{clientset: clientset}
	return s.k8sExecutor, nil
}

type k8sExecutor struct {
	clientset kubernetes.Interface
}

func (k *k8sExecutor) DeletePod(ctx context.Context, namespace, name string) error {
	return k.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (k *k8sExecutor) RestartDeployment(ctx context.Context, namespace, name string) error {
	patch := []byte(fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, time.Now().Format(time.RFC3339)))
	_, err := k.clientset.AppsV1().Deployments(namespace).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	return err
}

func (s *runtimeService) CreateRuntimeSpec(ctx context.Context, in CreateRuntimeSpecInput) (*domain.RuntimeSpec, error) {
	environment := strings.TrimSpace(in.Environment)
	if err := validateRuntimeSpecInput(in.ApplicationID, environment); err != nil {
		return nil, err
	}

	existing, err := s.repoStore().GetRuntimeSpecByApplicationEnv(ctx, in.ApplicationID, environment)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrDuplicateRuntimeSpec
	}

	now := time.Now().UTC()
	item := &domain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: in.ApplicationID,
		Environment:   environment,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repoStore().CreateRuntimeSpec(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *runtimeService) ListRuntimeSpecs(ctx context.Context) ([]*domain.RuntimeSpec, error) {
	return s.repoStore().ListRuntimeSpecs(ctx)
}

func (s *runtimeService) GetRuntimeSpec(ctx context.Context, id uuid.UUID) (*domain.RuntimeSpec, error) {
	if id == uuid.Nil {
		return nil, sharederrs.Required("id")
	}
	return s.repoStore().GetRuntimeSpec(ctx, id)
}

func (s *runtimeService) DeleteRuntimeSpecByApplicationEnv(ctx context.Context, applicationId uuid.UUID, environment string) error {
	environment = strings.TrimSpace(environment)
	if err := validateRuntimeSpecInput(applicationId, environment); err != nil {
		return err
	}

	existing, err := s.repoStore().GetRuntimeSpecByApplicationEnv(ctx, applicationId, environment)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrRuntimeSpecNotFound
	}
	if err := s.repoStore().DeleteRuntimeSpecByApplicationEnv(ctx, applicationId, environment); err != nil {
		if err == sql.ErrNoRows {
			return ErrRuntimeSpecNotFound
		}
		return err
	}
	return nil
}

func (s *runtimeService) CreateRuntimeSpecRevision(ctx context.Context, runtimeSpecID uuid.UUID, in CreateRuntimeSpecRevisionInput) (*domain.RuntimeSpecRevision, error) {
	if runtimeSpecID == uuid.Nil {
		return nil, sharederrs.Required("runtime_spec_id")
	}
	if _, err := s.repoStore().GetRuntimeSpec(ctx, runtimeSpecID); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrRuntimeSpecNotFound
		}
		return nil, err
	}

	nextRevision, err := s.repoStore().NextRevisionNumber(ctx, runtimeSpecID)
	if err != nil {
		return nil, err
	}

	item := &domain.RuntimeSpecRevision{
		ID:               uuid.New(),
		RuntimeSpecID:    runtimeSpecID,
		Revision:         nextRevision,
		Replicas:         in.Replicas,
		HealthThresholds: in.HealthThresholds,
		Resources:        in.Resources,
		Autoscaling:      in.Autoscaling,
		Scheduling:       in.Scheduling,
		PodEnvs:          in.PodEnvs,
		CreatedBy:        strings.TrimSpace(in.CreatedBy),
		CreatedAt:        time.Now().UTC(),
	}
	if err := s.repoStore().CreateRuntimeSpecRevision(ctx, item); err != nil {
		return nil, err
	}
	if err := s.repoStore().UpdateCurrentRevision(ctx, runtimeSpecID, item.ID); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *runtimeService) ListRuntimeSpecRevisions(ctx context.Context, runtimeSpecID uuid.UUID) ([]*domain.RuntimeSpecRevision, error) {
	if runtimeSpecID == uuid.Nil {
		return nil, sharederrs.Required("runtime_spec_id")
	}
	return s.repoStore().ListRuntimeSpecRevisions(ctx, runtimeSpecID)
}

func (s *runtimeService) GetRuntimeSpecRevision(ctx context.Context, id uuid.UUID) (*domain.RuntimeSpecRevision, error) {
	if id == uuid.Nil {
		return nil, sharederrs.Required("id")
	}
	return s.repoStore().GetRuntimeSpecRevision(ctx, id)
}

func (s *runtimeService) GetObservedWorkload(ctx context.Context, runtimeSpecID uuid.UUID) (*domain.RuntimeObservedWorkload, error) {
	if runtimeSpecID == uuid.Nil {
		return nil, sharederrs.Required("runtime_spec_id")
	}
	item, err := s.repoStore().GetObservedWorkload(ctx, runtimeSpecID)
	if err == sql.ErrNoRows {
		return nil, ErrRuntimeSpecNotFound
	}
	return item, err
}

func (s *runtimeService) GetObservedWorkloadByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment string) (*domain.RuntimeObservedWorkload, error) {
	environment = strings.TrimSpace(environment)
	if err := validateRuntimeSpecInput(applicationID, environment); err != nil {
		return nil, err
	}
	spec, err := s.repoStore().EnsureRuntimeSpecByApplicationEnv(ctx, applicationID, environment)
	if err != nil {
		return nil, err
	}
	if spec == nil {
		return nil, ErrRuntimeSpecNotFound
	}
	item, err := s.repoStore().GetObservedWorkload(ctx, spec.ID)
	if err == sql.ErrNoRows {
		return nil, ErrRuntimeSpecNotFound
	}
	return item, err
}

func (s *runtimeService) SyncObservedWorkload(ctx context.Context, in SyncObservedWorkloadInput) (*domain.RuntimeObservedWorkload, error) {
	in.Environment = strings.TrimSpace(in.Environment)
	in.Namespace = strings.TrimSpace(in.Namespace)
	in.WorkloadKind = strings.TrimSpace(in.WorkloadKind)
	in.WorkloadName = strings.TrimSpace(in.WorkloadName)
	in.SummaryStatus = strings.TrimSpace(in.SummaryStatus)
	if err := validateObservedWorkloadInput(in.ApplicationID, in.Environment, in.WorkloadKind, in.WorkloadName); err != nil {
		return nil, err
	}

	spec, err := s.repoStore().EnsureRuntimeSpecByApplicationEnv(ctx, in.ApplicationID, in.Environment)
	if err != nil {
		return nil, err
	}
	if spec == nil {
		return nil, ErrRuntimeSpecNotFound
	}

	observedAt := in.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	resolvedNamespace := resolveRuntimeNamespace(spec.ApplicationID, spec.Environment)
	if in.Namespace != "" && in.Namespace != resolvedNamespace {
		return nil, ErrNamespaceMismatch
	}

	item := &domain.RuntimeObservedWorkload{
		ID:                  uuid.New(),
		RuntimeSpecID:       spec.ID,
		ApplicationID:       spec.ApplicationID,
		Environment:         spec.Environment,
		Namespace:           resolvedNamespace,
		WorkloadKind:        in.WorkloadKind,
		WorkloadName:        in.WorkloadName,
		DesiredReplicas:     in.DesiredReplicas,
		ReadyReplicas:       in.ReadyReplicas,
		UpdatedReplicas:     in.UpdatedReplicas,
		AvailableReplicas:   in.AvailableReplicas,
		UnavailableReplicas: in.UnavailableReplicas,
		ObservedGeneration:  in.ObservedGeneration,
		SummaryStatus:       in.SummaryStatus,
		Images:              trimStringSlice(in.Images),
		Conditions:          mapObservedWorkloadConditions(in.Conditions),
		Labels:              copyLabels(in.Labels),
		Annotations:         copyLabels(in.Annotations),
		ObservedAt:          observedAt,
		RestartAt:           in.RestartAt,
	}
	if err := s.repoStore().UpsertObservedWorkload(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *runtimeService) DeleteObservedWorkload(ctx context.Context, in DeleteObservedWorkloadInput) error {
	in.Environment = strings.TrimSpace(in.Environment)
	in.Namespace = strings.TrimSpace(in.Namespace)
	in.WorkloadKind = strings.TrimSpace(in.WorkloadKind)
	in.WorkloadName = strings.TrimSpace(in.WorkloadName)
	if err := validateObservedWorkloadDeleteInput(in.ApplicationID, in.Environment, in.WorkloadKind, in.WorkloadName); err != nil {
		return err
	}

	spec, err := s.repoStore().EnsureRuntimeSpecByApplicationEnv(ctx, in.ApplicationID, in.Environment)
	if err != nil {
		return err
	}
	if spec == nil {
		return ErrRuntimeSpecNotFound
	}

	observedAt := in.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	resolvedNamespace := resolveRuntimeNamespace(spec.ApplicationID, spec.Environment)
	if in.Namespace != "" && in.Namespace != resolvedNamespace {
		return ErrNamespaceMismatch
	}
	return s.repoStore().DeleteObservedWorkload(ctx, spec.ID, resolvedNamespace, in.WorkloadKind, in.WorkloadName, observedAt)
}

func (s *runtimeService) ListObservedPods(ctx context.Context, runtimeSpecID uuid.UUID) ([]*domain.RuntimeObservedPod, error) {
	if runtimeSpecID == uuid.Nil {
		return nil, sharederrs.Required("runtime_spec_id")
	}
	return s.repoStore().ListObservedPods(ctx, runtimeSpecID)
}

func (s *runtimeService) ListObservedPodsByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment string) ([]*domain.RuntimeObservedPod, error) {
	environment = strings.TrimSpace(environment)
	if err := validateRuntimeSpecInput(applicationID, environment); err != nil {
		return nil, err
	}
	spec, err := s.repoStore().EnsureRuntimeSpecByApplicationEnv(ctx, applicationID, environment)
	if err != nil {
		return nil, err
	}
	if spec == nil {
		return nil, ErrRuntimeSpecNotFound
	}
	return s.repoStore().ListObservedPods(ctx, spec.ID)
}

func (s *runtimeService) SyncObservedPod(ctx context.Context, in SyncObservedPodInput) (*domain.RuntimeObservedPod, error) {
	in.Environment = strings.TrimSpace(in.Environment)
	in.Namespace = strings.TrimSpace(in.Namespace)
	in.PodName = strings.TrimSpace(in.PodName)
	in.Phase = strings.TrimSpace(in.Phase)
	if err := validateObservedPodInput(in.ApplicationID, in.Environment, in.PodName, in.Phase); err != nil {
		return nil, err
	}

	spec, err := s.repoStore().EnsureRuntimeSpecByApplicationEnv(ctx, in.ApplicationID, in.Environment)
	if err != nil {
		return nil, err
	}
	if spec == nil {
		return nil, ErrRuntimeSpecNotFound
	}

	observedAt := in.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	resolvedNamespace := resolveRuntimeNamespace(spec.ApplicationID, spec.Environment)
	if in.Namespace != "" && in.Namespace != resolvedNamespace {
		return nil, ErrNamespaceMismatch
	}

	item := &domain.RuntimeObservedPod{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: spec.ApplicationID,
		Environment:   spec.Environment,
		Namespace:     resolvedNamespace,
		PodName:       in.PodName,
		Phase:         in.Phase,
		Ready:         in.Ready,
		Restarts:      in.Restarts,
		NodeName:      strings.TrimSpace(in.NodeName),
		PodIP:         strings.TrimSpace(in.PodIP),
		HostIP:        strings.TrimSpace(in.HostIP),
		OwnerKind:     strings.TrimSpace(in.OwnerKind),
		OwnerName:     strings.TrimSpace(in.OwnerName),
		Labels:        copyLabels(in.Labels),
		Containers:    mapObservedPodContainers(in.Containers),
		ObservedAt:    observedAt,
	}
	if err := s.repoStore().UpsertObservedPod(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *runtimeService) DeleteObservedPod(ctx context.Context, in DeleteObservedPodInput) error {
	in.Environment = strings.TrimSpace(in.Environment)
	in.Namespace = strings.TrimSpace(in.Namespace)
	in.PodName = strings.TrimSpace(in.PodName)
	if err := validateObservedPodDeleteInput(in.ApplicationID, in.Environment, in.PodName); err != nil {
		return err
	}

	spec, err := s.repoStore().EnsureRuntimeSpecByApplicationEnv(ctx, in.ApplicationID, in.Environment)
	if err != nil {
		return err
	}
	if spec == nil {
		return ErrRuntimeSpecNotFound
	}

	observedAt := in.ObservedAt
	if observedAt.IsZero() {
		now := time.Now().UTC()
		observedAt = now
	}
	resolvedNamespace := resolveRuntimeNamespace(spec.ApplicationID, spec.Environment)
	if in.Namespace != "" && in.Namespace != resolvedNamespace {
		return ErrNamespaceMismatch
	}
	return s.repoStore().DeleteObservedPod(ctx, spec.ID, resolvedNamespace, in.PodName, observedAt)
}

func (s *runtimeService) DeletePod(ctx context.Context, runtimeSpecID uuid.UUID, podName, operator string) error {
	if runtimeSpecID == uuid.Nil {
		return sharederrs.Required("id")
	}
	if strings.TrimSpace(podName) == "" {
		return sharederrs.Required("pod_name")
	}

	spec, err := s.repoStore().GetRuntimeSpec(ctx, runtimeSpecID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrRuntimeSpecNotFound
		}
		return err
	}
	if spec == nil {
		return ErrRuntimeSpecNotFound
	}

	k8s, err := s.k8s()
	if err != nil {
		return err
	}

	namespace := resolveRuntimeNamespace(spec.ApplicationID, spec.Environment)
	if err := k8s.DeletePod(ctx, namespace, podName); err != nil {
		if apierrors.IsNotFound(err) {
			return ErrK8sNotFound
		}
		if apierrors.IsForbidden(err) {
			return ErrK8sForbidden
		}
		return err
	}

	return s.recordOperation(ctx, spec.ID, "pod_delete", podName, operator)
}

func (s *runtimeService) DeletePodByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment, podName, operator string) error {
	environment = strings.TrimSpace(environment)
	if err := validateRuntimeSpecInput(applicationID, environment); err != nil {
		return err
	}
	spec, err := s.repoStore().EnsureRuntimeSpecByApplicationEnv(ctx, applicationID, environment)
	if err != nil {
		return err
	}
	if spec == nil {
		return ErrRuntimeSpecNotFound
	}
	return s.DeletePod(ctx, spec.ID, podName, operator)
}

func (s *runtimeService) RestartDeployment(ctx context.Context, runtimeSpecID uuid.UUID, deploymentName, operator string) error {
	if runtimeSpecID == uuid.Nil {
		return sharederrs.Required("id")
	}
	if strings.TrimSpace(deploymentName) == "" {
		return sharederrs.Required("deployment_name")
	}

	spec, err := s.repoStore().GetRuntimeSpec(ctx, runtimeSpecID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrRuntimeSpecNotFound
		}
		return err
	}
	if spec == nil {
		return ErrRuntimeSpecNotFound
	}

	k8s, err := s.k8s()
	if err != nil {
		return err
	}

	namespace := resolveRuntimeNamespace(spec.ApplicationID, spec.Environment)
	if err := k8s.RestartDeployment(ctx, namespace, deploymentName); err != nil {
		if apierrors.IsNotFound(err) {
			return ErrK8sNotFound
		}
		if apierrors.IsForbidden(err) {
			return ErrK8sForbidden
		}
		return err
	}

	return s.recordOperation(ctx, spec.ID, "deployment_restart", deploymentName, operator)
}

func (s *runtimeService) RestartDeploymentByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment, deploymentName, operator string) error {
	environment = strings.TrimSpace(environment)
	if err := validateRuntimeSpecInput(applicationID, environment); err != nil {
		return err
	}
	spec, err := s.repoStore().EnsureRuntimeSpecByApplicationEnv(ctx, applicationID, environment)
	if err != nil {
		return err
	}
	if spec == nil {
		return ErrRuntimeSpecNotFound
	}
	return s.RestartDeployment(ctx, spec.ID, deploymentName, operator)
}

func (s *runtimeService) ListRuntimeOperations(ctx context.Context, runtimeSpecID uuid.UUID) ([]*domain.RuntimeOperation, error) {
	if runtimeSpecID == uuid.Nil {
		return nil, sharederrs.Required("id")
	}
	return s.repoStore().ListRuntimeOperations(ctx, runtimeSpecID)
}

func (s *runtimeService) recordOperation(ctx context.Context, runtimeSpecID uuid.UUID, operationType, targetName, operator string) error {
	op := &domain.RuntimeOperation{
		ID:            uuid.New(),
		RuntimeSpecID: runtimeSpecID,
		OperationType: operationType,
		TargetName:    targetName,
		Operator:      strings.TrimSpace(operator),
		CreatedAt:     time.Now().UTC(),
	}
	if err := s.repoStore().CreateRuntimeOperation(ctx, op); err != nil {
		return err
	}
	return nil
}

func validateRuntimeSpecInput(applicationId uuid.UUID, environment string) error {
	messages := make([]string, 0, 2)
	if applicationId == uuid.Nil {
		messages = append(messages, "application_id is required")
	}
	if environment == "" {
		messages = append(messages, "environment is required")
	}
	return sharederrs.JoinInvalid(messages)
}

func validateObservedPodInput(applicationId uuid.UUID, environment, podName, phase string) error {
	messages := make([]string, 0, 4)
	if applicationId == uuid.Nil {
		messages = append(messages, "application_id is required")
	}
	if environment == "" {
		messages = append(messages, "environment is required")
	}
	if podName == "" {
		messages = append(messages, "pod_name is required")
	}
	if phase == "" {
		messages = append(messages, "phase is required")
	}
	return sharederrs.JoinInvalid(messages)
}

func validateObservedPodDeleteInput(applicationId uuid.UUID, environment, podName string) error {
	messages := make([]string, 0, 3)
	if applicationId == uuid.Nil {
		messages = append(messages, "application_id is required")
	}
	if environment == "" {
		messages = append(messages, "environment is required")
	}
	if podName == "" {
		messages = append(messages, "pod_name is required")
	}
	return sharederrs.JoinInvalid(messages)
}

func validateObservedWorkloadInput(applicationId uuid.UUID, environment, workloadKind, workloadName string) error {
	messages := make([]string, 0, 4)
	if applicationId == uuid.Nil {
		messages = append(messages, "application_id is required")
	}
	if environment == "" {
		messages = append(messages, "environment is required")
	}
	if workloadKind == "" {
		messages = append(messages, "workload_kind is required")
	}
	if workloadName == "" {
		messages = append(messages, "workload_name is required")
	}
	return sharederrs.JoinInvalid(messages)
}

func validateObservedWorkloadDeleteInput(applicationId uuid.UUID, environment, workloadKind, workloadName string) error {
	return validateObservedWorkloadInput(applicationId, environment, workloadKind, workloadName)
}

func mapObservedPodContainers(in []ObservedPodContainerInput) []domain.RuntimeObservedPodContainer {
	if len(in) == 0 {
		return nil
	}
	out := make([]domain.RuntimeObservedPodContainer, 0, len(in))
	for _, item := range in {
		out = append(out, domain.RuntimeObservedPodContainer{
			Name:         strings.TrimSpace(item.Name),
			Image:        strings.TrimSpace(item.Image),
			ImageID:      strings.TrimSpace(item.ImageID),
			Ready:        item.Ready,
			RestartCount: item.RestartCount,
			State:        strings.TrimSpace(item.State),
		})
	}
	return out
}

func mapObservedWorkloadConditions(in []ObservedWorkloadConditionInput) []domain.RuntimeObservedWorkloadCondition {
	if len(in) == 0 {
		return nil
	}
	out := make([]domain.RuntimeObservedWorkloadCondition, 0, len(in))
	for _, item := range in {
		out = append(out, domain.RuntimeObservedWorkloadCondition{
			Type:               strings.TrimSpace(item.Type),
			Status:             strings.TrimSpace(item.Status),
			Reason:             strings.TrimSpace(item.Reason),
			Message:            strings.TrimSpace(item.Message),
			LastTransitionTime: item.LastTransitionTime,
		})
	}
	return out
}

func copyLabels(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}

func trimStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, item := range in {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func deriveRuntimeNamespace(applicationId uuid.UUID, environment string) string {
	base := applicationId.String()
	environment = strings.ToLower(strings.TrimSpace(environment))
	if environment == "" || environment == "production" {
		return base
	}
	return base + "-" + environment
}

func resolveRuntimeNamespace(applicationId uuid.UUID, environment string) string {
	if ns := strings.TrimSpace(os.Getenv("POD_NAMESPACE")); ns != "" {
		return ns
	}
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); ns != "" {
			return ns
		}
	}
	return deriveRuntimeNamespace(applicationId, environment)
}
