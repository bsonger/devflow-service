package repository

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"sync"
	"time"

	runtimedomain "github.com/bsonger/devflow-service/internal/runtime/domain"
	"github.com/google/uuid"
)

type memoryStore struct {
	mu sync.RWMutex

	runtimeSpecs         map[uuid.UUID]*runtimedomain.RuntimeSpec
	runtimeSpecByAppEnv  map[string]uuid.UUID
	runtimeSpecRevisions map[uuid.UUID][]*runtimedomain.RuntimeSpecRevision
	observedWorkloads    map[uuid.UUID]*runtimedomain.RuntimeObservedWorkload
	observedPods         map[uuid.UUID]map[string]*runtimedomain.RuntimeObservedPod
	runtimeOperations    map[uuid.UUID][]*runtimedomain.RuntimeOperation
}

func NewMemoryStore() Store {
	return &memoryStore{
		runtimeSpecs:         map[uuid.UUID]*runtimedomain.RuntimeSpec{},
		runtimeSpecByAppEnv:  map[string]uuid.UUID{},
		runtimeSpecRevisions: map[uuid.UUID][]*runtimedomain.RuntimeSpecRevision{},
		observedWorkloads:    map[uuid.UUID]*runtimedomain.RuntimeObservedWorkload{},
		observedPods:         map[uuid.UUID]map[string]*runtimedomain.RuntimeObservedPod{},
		runtimeOperations:    map[uuid.UUID][]*runtimedomain.RuntimeOperation{},
	}
}

func (s *memoryStore) CreateRuntimeSpec(_ context.Context, spec *runtimedomain.RuntimeSpec) error {
	if spec == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := runtimeAppEnvKey(spec.ApplicationID, spec.Environment)
	s.runtimeSpecs[spec.ID] = cloneRuntimeSpec(spec)
	s.runtimeSpecByAppEnv[key] = spec.ID
	return nil
}

func (s *memoryStore) GetRuntimeSpec(_ context.Context, id uuid.UUID) (*runtimedomain.RuntimeSpec, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	spec := s.runtimeSpecs[id]
	if spec == nil {
		return nil, sql.ErrNoRows
	}
	return cloneRuntimeSpec(spec), nil
}

func (s *memoryStore) GetRuntimeSpecByApplicationEnv(_ context.Context, applicationID uuid.UUID, environment string) (*runtimedomain.RuntimeSpec, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.runtimeSpecByAppEnv[runtimeAppEnvKey(applicationID, environment)]
	if !ok {
		return nil, nil
	}
	spec := s.runtimeSpecs[id]
	if spec == nil {
		return nil, nil
	}
	return cloneRuntimeSpec(spec), nil
}

func (s *memoryStore) EnsureRuntimeSpecByApplicationEnv(_ context.Context, applicationID uuid.UUID, environment string) (*runtimedomain.RuntimeSpec, error) {
	environment = strings.TrimSpace(environment)
	key := runtimeAppEnvKey(applicationID, environment)

	s.mu.Lock()
	defer s.mu.Unlock()

	if id, ok := s.runtimeSpecByAppEnv[key]; ok {
		if spec := s.runtimeSpecs[id]; spec != nil {
			specCopy := cloneRuntimeSpec(spec)
			return specCopy, nil
		}
	}

	now := time.Now().UTC()
	spec := &runtimedomain.RuntimeSpec{
		ID:            uuid.New(),
		ApplicationID: applicationID,
		Environment:   environment,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.runtimeSpecs[spec.ID] = cloneRuntimeSpec(spec)
	s.runtimeSpecByAppEnv[key] = spec.ID
	return cloneRuntimeSpec(spec), nil
}

func (s *memoryStore) GetApplicationName(_ context.Context, applicationID uuid.UUID) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, spec := range s.runtimeSpecs {
		if spec == nil || spec.ApplicationID != applicationID {
			continue
		}
		if workload := s.observedWorkloads[spec.ID]; workload != nil {
			if name := strings.TrimSpace(workload.WorkloadName); name != "" {
				return name, nil
			}
		}
	}
	return "", sql.ErrNoRows
}

func (s *memoryStore) ResolveTargetNamespace(_ context.Context, applicationID uuid.UUID, environment string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.runtimeSpecByAppEnv[runtimeAppEnvKey(applicationID, environment)]
	if !ok {
		return "", sql.ErrNoRows
	}
	if workload := s.observedWorkloads[id]; workload != nil {
		if namespace := strings.TrimSpace(workload.Namespace); namespace != "" {
			return namespace, nil
		}
	}
	for _, pod := range s.observedPods[id] {
		if pod == nil {
			continue
		}
		if namespace := strings.TrimSpace(pod.Namespace); namespace != "" {
			return namespace, nil
		}
	}
	return "", sql.ErrNoRows
}

func (s *memoryStore) DeleteRuntimeSpecByApplicationEnv(_ context.Context, applicationID uuid.UUID, environment string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := runtimeAppEnvKey(applicationID, environment)
	id, ok := s.runtimeSpecByAppEnv[key]
	if !ok {
		return sql.ErrNoRows
	}
	delete(s.runtimeSpecByAppEnv, key)
	delete(s.runtimeSpecs, id)
	delete(s.runtimeSpecRevisions, id)
	delete(s.observedWorkloads, id)
	delete(s.observedPods, id)
	delete(s.runtimeOperations, id)
	return nil
}

func (s *memoryStore) ListRuntimeSpecs(_ context.Context) ([]*runtimedomain.RuntimeSpec, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]*runtimedomain.RuntimeSpec, 0, len(s.runtimeSpecs))
	for _, item := range s.runtimeSpecs {
		if item == nil {
			continue
		}
		items = append(items, cloneRuntimeSpec(item))
	}
	slices.SortFunc(items, func(a, b *runtimedomain.RuntimeSpec) int {
		return b.CreatedAt.Compare(a.CreatedAt)
	})
	return items, nil
}

func (s *memoryStore) NextRevisionNumber(_ context.Context, runtimeSpecID uuid.UUID) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.runtimeSpecRevisions[runtimeSpecID]) + 1, nil
}

func (s *memoryStore) CreateRuntimeSpecRevision(_ context.Context, revision *runtimedomain.RuntimeSpecRevision) error {
	if revision == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtimeSpecRevisions[revision.RuntimeSpecID] = append(
		s.runtimeSpecRevisions[revision.RuntimeSpecID],
		cloneRuntimeSpecRevision(revision),
	)
	return nil
}

func (s *memoryStore) UpdateCurrentRevision(_ context.Context, runtimeSpecID uuid.UUID, revisionID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	spec := s.runtimeSpecs[runtimeSpecID]
	if spec == nil {
		return sql.ErrNoRows
	}
	spec.CurrentRevisionID = &revisionID
	spec.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *memoryStore) ListRuntimeSpecRevisions(_ context.Context, runtimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeSpecRevision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.runtimeSpecRevisions[runtimeSpecID]
	out := make([]*runtimedomain.RuntimeSpecRevision, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, cloneRuntimeSpecRevision(item))
	}
	slices.SortFunc(out, func(a, b *runtimedomain.RuntimeSpecRevision) int {
		if a.Revision == b.Revision {
			return 0
		}
		if a.Revision < b.Revision {
			return -1
		}
		return 1
	})
	return out, nil
}

func (s *memoryStore) GetRuntimeSpecRevision(_ context.Context, id uuid.UUID) (*runtimedomain.RuntimeSpecRevision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, items := range s.runtimeSpecRevisions {
		for _, item := range items {
			if item != nil && item.ID == id {
				return cloneRuntimeSpecRevision(item), nil
			}
		}
	}
	return nil, sql.ErrNoRows
}

func (s *memoryStore) UpsertObservedWorkload(_ context.Context, workload *runtimedomain.RuntimeObservedWorkload) error {
	if workload == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observedWorkloads[workload.RuntimeSpecID] = cloneObservedWorkload(workload)
	return nil
}

func (s *memoryStore) DeleteObservedWorkload(_ context.Context, runtimeSpecID uuid.UUID, namespace, workloadKind, workloadName string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.observedWorkloads[runtimeSpecID]
	if item == nil {
		return sql.ErrNoRows
	}
	if strings.TrimSpace(namespace) != "" && strings.TrimSpace(item.Namespace) != strings.TrimSpace(namespace) {
		return sql.ErrNoRows
	}
	if strings.TrimSpace(workloadKind) != "" && !strings.EqualFold(strings.TrimSpace(item.WorkloadKind), strings.TrimSpace(workloadKind)) {
		return sql.ErrNoRows
	}
	if strings.TrimSpace(workloadName) != "" && strings.TrimSpace(item.WorkloadName) != strings.TrimSpace(workloadName) {
		return sql.ErrNoRows
	}
	delete(s.observedWorkloads, runtimeSpecID)
	return nil
}

func (s *memoryStore) GetObservedWorkload(_ context.Context, runtimeSpecID uuid.UUID) (*runtimedomain.RuntimeObservedWorkload, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item := s.observedWorkloads[runtimeSpecID]
	if item == nil {
		return nil, sql.ErrNoRows
	}
	return cloneObservedWorkload(item), nil
}

func (s *memoryStore) UpsertObservedPod(_ context.Context, pod *runtimedomain.RuntimeObservedPod) error {
	if pod == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.observedPods[pod.RuntimeSpecID] == nil {
		s.observedPods[pod.RuntimeSpecID] = map[string]*runtimedomain.RuntimeObservedPod{}
	}
	s.observedPods[pod.RuntimeSpecID][pod.PodName] = cloneObservedPod(pod)
	return nil
}

func (s *memoryStore) DeleteObservedPod(_ context.Context, runtimeSpecID uuid.UUID, namespace, podName string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pods := s.observedPods[runtimeSpecID]
	if len(pods) == 0 {
		return sql.ErrNoRows
	}
	item := pods[podName]
	if item == nil {
		return sql.ErrNoRows
	}
	if strings.TrimSpace(namespace) != "" && strings.TrimSpace(item.Namespace) != strings.TrimSpace(namespace) {
		return sql.ErrNoRows
	}
	delete(pods, podName)
	if len(pods) == 0 {
		delete(s.observedPods, runtimeSpecID)
	}
	return nil
}

func (s *memoryStore) ListObservedPods(_ context.Context, runtimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeObservedPod, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pods := s.observedPods[runtimeSpecID]
	out := make([]*runtimedomain.RuntimeObservedPod, 0, len(pods))
	for _, item := range pods {
		if item == nil {
			continue
		}
		out = append(out, cloneObservedPod(item))
	}
	slices.SortFunc(out, func(a, b *runtimedomain.RuntimeObservedPod) int {
		return strings.Compare(a.PodName, b.PodName)
	})
	return out, nil
}

func (s *memoryStore) CreateRuntimeOperation(_ context.Context, op *runtimedomain.RuntimeOperation) error {
	if op == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtimeOperations[op.RuntimeSpecID] = append(s.runtimeOperations[op.RuntimeSpecID], cloneRuntimeOperation(op))
	return nil
}

func (s *memoryStore) ListRuntimeOperations(_ context.Context, runtimeSpecID uuid.UUID) ([]*runtimedomain.RuntimeOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.runtimeOperations[runtimeSpecID]
	out := make([]*runtimedomain.RuntimeOperation, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, cloneRuntimeOperation(item))
	}
	slices.SortFunc(out, func(a, b *runtimedomain.RuntimeOperation) int {
		return b.CreatedAt.Compare(a.CreatedAt)
	})
	return out, nil
}

func runtimeAppEnvKey(applicationID uuid.UUID, environment string) string {
	return applicationID.String() + "|" + strings.TrimSpace(environment)
}

func cloneRuntimeSpec(in *runtimedomain.RuntimeSpec) *runtimedomain.RuntimeSpec {
	if in == nil {
		return nil
	}
	out := *in
	if in.CurrentRevisionID != nil {
		revisionID := *in.CurrentRevisionID
		out.CurrentRevisionID = &revisionID
	}
	return &out
}

func cloneRuntimeSpecRevision(in *runtimedomain.RuntimeSpecRevision) *runtimedomain.RuntimeSpecRevision {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneObservedWorkload(in *runtimedomain.RuntimeObservedWorkload) *runtimedomain.RuntimeObservedWorkload {
	if in == nil {
		return nil
	}
	out := *in
	out.Images = slices.Clone(in.Images)
	out.Conditions = slices.Clone(in.Conditions)
	out.Labels = cloneStringMap(in.Labels)
	out.Annotations = cloneStringMap(in.Annotations)
	if in.RestartAt != nil {
		restartAt := *in.RestartAt
		out.RestartAt = &restartAt
	}
	if in.DeletedAt != nil {
		deletedAt := *in.DeletedAt
		out.DeletedAt = &deletedAt
	}
	return &out
}

func cloneObservedPod(in *runtimedomain.RuntimeObservedPod) *runtimedomain.RuntimeObservedPod {
	if in == nil {
		return nil
	}
	out := *in
	out.Labels = cloneStringMap(in.Labels)
	out.Containers = slices.Clone(in.Containers)
	if in.DeletedAt != nil {
		deletedAt := *in.DeletedAt
		out.DeletedAt = &deletedAt
	}
	return &out
}

func cloneRuntimeOperation(in *runtimedomain.RuntimeOperation) *runtimedomain.RuntimeOperation {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
