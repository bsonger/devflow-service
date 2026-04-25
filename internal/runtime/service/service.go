package service

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/bsonger/devflow-service/internal/runtime/repository"
	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
	"github.com/google/uuid"
)

var ErrDuplicateRuntimeSpec = sharederrs.Conflict("runtime spec already exists for application and environment")
var ErrRuntimeSpecNotFound = sharederrs.NotFound("runtime spec not found")
var ErrNamespaceMismatch = sharederrs.InvalidArgument("observed pod namespace does not match derived runtime namespace")

type Service interface {
	CreateRuntimeSpec(context.Context, CreateRuntimeSpecInput) (*domain.RuntimeSpec, error)
	ListRuntimeSpecs(context.Context) ([]*domain.RuntimeSpec, error)
	GetRuntimeSpec(context.Context, uuid.UUID) (*domain.RuntimeSpec, error)
	DeleteRuntimeSpecByApplicationEnv(context.Context, uuid.UUID, string) error
	CreateRuntimeSpecRevision(context.Context, uuid.UUID, CreateRuntimeSpecRevisionInput) (*domain.RuntimeSpecRevision, error)
	ListRuntimeSpecRevisions(context.Context, uuid.UUID) ([]*domain.RuntimeSpecRevision, error)
	GetRuntimeSpecRevision(context.Context, uuid.UUID) (*domain.RuntimeSpecRevision, error)
	ListObservedPods(context.Context, uuid.UUID) ([]*domain.RuntimeObservedPod, error)
	SyncObservedPod(context.Context, SyncObservedPodInput) (*domain.RuntimeObservedPod, error)
	DeleteObservedPod(context.Context, DeleteObservedPodInput) error
}

type runtimeService struct {
	store repository.Store
}

var DefaultService Service = New(repository.NewPostgresStore())

func New(store repository.Store) Service {
	return &runtimeService{store: store}
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

func (s *runtimeService) DeleteRuntimeSpecByApplicationEnv(ctx context.Context, applicationID uuid.UUID, environment string) error {
	environment = strings.TrimSpace(environment)
	if err := validateRuntimeSpecInput(applicationID, environment); err != nil {
		return err
	}

	existing, err := s.repoStore().GetRuntimeSpecByApplicationEnv(ctx, applicationID, environment)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrRuntimeSpecNotFound
	}
	if err := s.repoStore().DeleteRuntimeSpecByApplicationEnv(ctx, applicationID, environment); err != nil {
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

func (s *runtimeService) ListObservedPods(ctx context.Context, runtimeSpecID uuid.UUID) ([]*domain.RuntimeObservedPod, error) {
	if runtimeSpecID == uuid.Nil {
		return nil, sharederrs.Required("runtime_spec_id")
	}
	return s.repoStore().ListObservedPods(ctx, runtimeSpecID)
}

func (s *runtimeService) SyncObservedPod(ctx context.Context, in SyncObservedPodInput) (*domain.RuntimeObservedPod, error) {
	in.Environment = strings.TrimSpace(in.Environment)
	in.Namespace = strings.TrimSpace(in.Namespace)
	in.PodName = strings.TrimSpace(in.PodName)
	in.Phase = strings.TrimSpace(in.Phase)
	if err := validateObservedPodInput(in.ApplicationID, in.Environment, in.PodName, in.Phase); err != nil {
		return nil, err
	}

	spec, err := s.repoStore().GetRuntimeSpecByApplicationEnv(ctx, in.ApplicationID, in.Environment)
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
	derivedNamespace := deriveRuntimeNamespace(spec.ApplicationID, spec.Environment)
	if in.Namespace != "" && in.Namespace != derivedNamespace {
		return nil, ErrNamespaceMismatch
	}

	item := &domain.RuntimeObservedPod{
		ID:            uuid.New(),
		RuntimeSpecID: spec.ID,
		ApplicationID: spec.ApplicationID,
		Environment:   spec.Environment,
		Namespace:     derivedNamespace,
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

	spec, err := s.repoStore().GetRuntimeSpecByApplicationEnv(ctx, in.ApplicationID, in.Environment)
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
	derivedNamespace := deriveRuntimeNamespace(spec.ApplicationID, spec.Environment)
	if in.Namespace != "" && in.Namespace != derivedNamespace {
		return ErrNamespaceMismatch
	}
	return s.repoStore().DeleteObservedPod(ctx, spec.ID, derivedNamespace, in.PodName, observedAt)
}

func validateRuntimeSpecInput(applicationID uuid.UUID, environment string) error {
	messages := make([]string, 0, 2)
	if applicationID == uuid.Nil {
		messages = append(messages, "application_id is required")
	}
	if environment == "" {
		messages = append(messages, "environment is required")
	}
	return sharederrs.JoinInvalid(messages)
}

func validateObservedPodInput(applicationID uuid.UUID, environment, podName, phase string) error {
	messages := make([]string, 0, 4)
	if applicationID == uuid.Nil {
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

func validateObservedPodDeleteInput(applicationID uuid.UUID, environment, podName string) error {
	messages := make([]string, 0, 3)
	if applicationID == uuid.Nil {
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

func deriveRuntimeNamespace(applicationID uuid.UUID, environment string) string {
	base := applicationID.String()
	environment = strings.ToLower(strings.TrimSpace(environment))
	if environment == "" || environment == "production" {
		return base
	}
	return base + "-" + environment
}
