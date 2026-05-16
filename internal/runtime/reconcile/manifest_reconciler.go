package reconcile

import (
	"context"
	"strings"

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
}

func NewManifestReconciler(source ManifestSnapshotSource, writer ManifestWriter) *ManifestReconciler {
	return &ManifestReconciler{
		source: source,
		writer: writer,
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
		return nil
	}

	return r.writer.WriteResult(ctx, ManifestResultWrite{
		ManifestID:  manifestID,
		PipelineID:  pipelineID,
		CommitHash:  result.CommitHash,
		ImageRef:    result.ImageRef,
		ImageTag:    result.ImageTag,
		ImageDigest: result.ImageDigest,
	})
}
