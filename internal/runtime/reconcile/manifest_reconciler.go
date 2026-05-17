package reconcile

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/runtime/watch"
)

type ManifestSnapshotSource interface {
	GetManifestSnapshot(manifestID string) (*watch.TektonSnapshot, bool)
}

type ManifestStatusWrite struct {
	ManifestID string
	PipelineID string
	Status     releasedomain.ManifestStatus
	Message    string
}

type ManifestTaskWrite struct {
	ManifestID string
	PipelineID string
	TaskName   string
	TaskRun    string
	Status     releasedomain.StepStatus
	Message    string
}

type ManifestResultWrite struct {
	ManifestID  string
	PipelineID  string
	CommitHash  string
	ImageRef    string
	ImageTag    string
	ImageDigest string
}

type ManifestWriter interface {
	WriteStatus(context.Context, ManifestStatusWrite) error
	WriteTask(context.Context, ManifestTaskWrite) error
	WriteResult(context.Context, ManifestResultWrite) error
}

type ManifestReconciler struct {
	source ManifestSnapshotSource
	writer ManifestWriter
	mu     sync.Mutex
	last   map[string]string
}

func NewManifestReconciler(source ManifestSnapshotSource, writer ManifestWriter) *ManifestReconciler {
	return &ManifestReconciler{
		source: source,
		writer: writer,
		last:   map[string]string{},
	}
}

func (r *ManifestReconciler) Reconcile(ctx context.Context, manifestID string) error {
	if r == nil || r.source == nil || r.writer == nil {
		return nil
	}

	manifestID = strings.TrimSpace(manifestID)
	if manifestID == "" {
		return nil
	}

	snapshot, ok := r.source.GetManifestSnapshot(manifestID)
	if !ok || snapshot == nil || len(snapshot.PipelineRuns) == 0 {
		return nil
	}

	pipeline := snapshot.PipelineRuns[0]
	pipelineID := strings.TrimSpace(pipeline.Name)
	if pipelineID == "" {
		return nil
	}
	stateKey := manifestSnapshotStateKey(pipeline, snapshot.TaskRuns[manifestID])
	if r.isProcessed(manifestID, stateKey) {
		return nil
	}

	if err := r.writer.WriteStatus(ctx, ManifestStatusWrite{
		ManifestID: manifestID,
		PipelineID: pipelineID,
		Status:     releasedomain.ManifestStatus(strings.TrimSpace(pipeline.Status)),
		Message:    strings.TrimSpace(pipeline.Message),
	}); err != nil {
		return err
	}

	taskSnapshots := snapshot.TaskRuns[manifestID]
	for _, task := range taskSnapshots {
		if err := r.writer.WriteTask(ctx, ManifestTaskWrite{
			ManifestID: manifestID,
			PipelineID: pipelineID,
			TaskName:   strings.TrimSpace(task.TaskName),
			TaskRun:    strings.TrimSpace(task.TaskRun),
			Status:     releasedomain.StepStatus(strings.TrimSpace(task.Status)),
			Message:    strings.TrimSpace(task.Message),
		}); err != nil {
			return err
		}
	}

	result := watch.BuildManifestResultSnapshot(taskSnapshots)
	if result == nil {
		r.markProcessed(manifestID, stateKey)
		return nil
	}

	if err := r.writer.WriteResult(ctx, ManifestResultWrite{
		ManifestID:  manifestID,
		PipelineID:  pipelineID,
		CommitHash:  result.CommitHash,
		ImageRef:    result.ImageRef,
		ImageTag:    result.ImageTag,
		ImageDigest: result.ImageDigest,
	}); err != nil {
		return err
	}
	r.markProcessed(manifestID, stateKey)
	return nil
}

func (r *ManifestReconciler) isProcessed(manifestID, stateKey string) bool {
	if r == nil {
		return false
	}
	manifestID = strings.TrimSpace(manifestID)
	stateKey = strings.TrimSpace(stateKey)
	if manifestID == "" || stateKey == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last[manifestID] == stateKey
}

func (r *ManifestReconciler) markProcessed(manifestID, stateKey string) {
	if r == nil {
		return
	}
	manifestID = strings.TrimSpace(manifestID)
	stateKey = strings.TrimSpace(stateKey)
	if manifestID == "" || stateKey == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last[manifestID] = stateKey
}

func manifestSnapshotStateKey(pipeline watch.PipelineRunSnapshot, tasks []watch.TaskRunSnapshot) string {
	parts := []string{
		strings.TrimSpace(pipeline.StateKey),
		strings.TrimSpace(pipeline.Status),
		strings.TrimSpace(pipeline.Message),
	}
	taskKeys := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskKeys = append(taskKeys, fmt.Sprintf("%s|%s|%s|%s",
			strings.TrimSpace(task.TaskName),
			strings.TrimSpace(task.TaskRun),
			strings.TrimSpace(task.Status),
			strings.TrimSpace(task.Message),
		))
	}
	sort.Strings(taskKeys)
	parts = append(parts, taskKeys...)
	if result := watch.BuildManifestResultSnapshot(tasks); result != nil {
		parts = append(parts,
			strings.TrimSpace(result.CommitHash),
			strings.TrimSpace(result.ImageRef),
			strings.TrimSpace(result.ImageTag),
			strings.TrimSpace(result.ImageDigest),
		)
	}
	return strings.Join(parts, "\n")
}
