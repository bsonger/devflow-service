package reconcile

import (
	"context"
	"reflect"
	"testing"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/bsonger/devflow-service/internal/runtime/watch"
)

func TestManifestReconcilerPostsStatusTasksAndResult(t *testing.T) {
	writer := &stubManifestWriter{}
	reconciler := NewManifestReconciler(
		stubTektonSnapshotSource{
			snapshot: &watch.TektonSnapshot{
				PipelineRuns: []watch.PipelineRunSnapshot{
					{
						ManifestID: "m-1",
						Name:       "pr-1",
						Status:     string(releasedomain.ManifestAvailable),
						Message:    "pipeline completed",
					},
					{
						ManifestID: "m-1",
						Name:       "pr-older",
						Status:     string(releasedomain.ManifestPending),
						Message:    "older pipeline",
					},
				},
				TaskRuns: map[string][]watch.TaskRunSnapshot{
					"m-1": {
						{
							ManifestID: "m-1",
							PipelineID: "pr-1",
							TaskName:   "build-image",
							TaskRun:    "tr-1",
							Status:     string(releasedomain.StepSucceeded),
							Message:    "build completed",
							Params: map[string]string{
								"IMAGE_REPOSITORY": "registry.example.com/demo",
								"IMAGE_TAG":        "v1",
							},
							Results: map[string]string{
								"IMAGE_TAG":    "v1",
								"IMAGE_DIGEST": "sha256:abc",
							},
						},
						{
							ManifestID: "m-1",
							PipelineID: "pr-1",
							TaskName:   "push-image",
							TaskRun:    "tr-2",
							Status:     string(releasedomain.StepSucceeded),
							Message:    "",
							Results: map[string]string{
								"commit": "abc123",
							},
						},
					},
				},
			},
		},
		writer,
	)

	if err := reconciler.Reconcile(context.Background(), " m-1 "); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if len(writer.statuses) != 1 {
		t.Fatalf("statuses = %d, want 1", len(writer.statuses))
	}
	if got := writer.statuses[0]; got.ManifestID != "m-1" || got.PipelineID != "pr-1" || got.Status != releasedomain.ManifestAvailable || got.Message != "pipeline completed" {
		t.Fatalf("status write = %#v", got)
	}
	if len(writer.tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(writer.tasks))
	}
	if got := writer.tasks[0]; got.ManifestID != "m-1" || got.PipelineID != "pr-1" || got.TaskName != "build-image" || got.TaskRun != "tr-1" || got.Status != releasedomain.StepSucceeded || got.Message != "build completed" {
		t.Fatalf("first task write = %#v", got)
	}
	if got := writer.tasks[1]; got.ManifestID != "m-1" || got.PipelineID != "pr-1" || got.TaskName != "push-image" || got.TaskRun != "tr-2" || got.Status != releasedomain.StepSucceeded || got.Message != "" {
		t.Fatalf("second task write = %#v", got)
	}
	if len(writer.results) != 1 {
		t.Fatalf("results = %d, want 1", len(writer.results))
	}
	if got := writer.results[0]; got.ManifestID != "m-1" || got.PipelineID != "pr-1" || got.CommitHash != "abc123" || got.ImageRef != "registry.example.com/demo@sha256:abc" || got.ImageTag != "v1" || got.ImageDigest != "sha256:abc" {
		t.Fatalf("result write = %#v", got)
	}
	if !reflect.DeepEqual(writer.callOrder, []string{"status", "task", "task", "result"}) {
		t.Fatalf("call order = %#v", writer.callOrder)
	}
}

func TestManifestReconcilerNoopsWhenSnapshotMissing(t *testing.T) {
	writer := &stubManifestWriter{}
	reconciler := NewManifestReconciler(stubTektonSnapshotSource{}, writer)

	if err := reconciler.Reconcile(context.Background(), " "); err != nil {
		t.Fatalf("Reconcile with empty manifestID failed: %v", err)
	}
	if err := reconciler.Reconcile(context.Background(), "missing"); err != nil {
		t.Fatalf("Reconcile with missing snapshot failed: %v", err)
	}
	if len(writer.statuses) != 0 || len(writer.tasks) != 0 || len(writer.results) != 0 {
		t.Fatalf("writer recorded unexpected calls: statuses=%d tasks=%d results=%d", len(writer.statuses), len(writer.tasks), len(writer.results))
	}
}

type stubTektonSnapshotSource struct {
	snapshot *watch.TektonSnapshot
}

func (s stubTektonSnapshotSource) GetManifestSnapshot(string) (*watch.TektonSnapshot, bool) {
	if s.snapshot == nil {
		return nil, false
	}
	return s.snapshot, true
}

type stubManifestWriter struct {
	statuses  []ManifestStatusWrite
	tasks     []ManifestTaskWrite
	results   []ManifestResultWrite
	callOrder []string
}

func (s *stubManifestWriter) WriteStatus(_ context.Context, input ManifestStatusWrite) error {
	s.statuses = append(s.statuses, input)
	s.callOrder = append(s.callOrder, "status")
	return nil
}

func (s *stubManifestWriter) WriteTask(_ context.Context, input ManifestTaskWrite) error {
	s.tasks = append(s.tasks, input)
	s.callOrder = append(s.callOrder, "task")
	return nil
}

func (s *stubManifestWriter) WriteResult(_ context.Context, input ManifestResultWrite) error {
	s.results = append(s.results, input)
	s.callOrder = append(s.callOrder, "result")
	return nil
}
