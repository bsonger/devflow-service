package watch

import (
	"context"
	"errors"
	"testing"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestTektonSnapshotListsOnlyMatchingControlPlaneManifests(t *testing.T) {
	cache := &tektonCache{
		pipelineRuns: []PipelineRunSnapshot{
			{ManifestID: "m-1", Name: "pr-1", ControlPlaneID: "cp-1"},
			{ManifestID: "m-2", Name: "pr-2", ControlPlaneID: "cp-2"},
			{ManifestID: "m-1", Name: "pr-3", ControlPlaneID: "cp-1"},
			{ManifestID: "", Name: "pr-4", ControlPlaneID: "cp-1"},
		},
	}

	items := cache.ListManifestIDs("cp-1")
	if len(items) != 1 || items[0] != "m-1" {
		t.Fatalf("items = %#v, want [m-1]", items)
	}
}

func TestTektonSnapshotGetManifestSnapshot(t *testing.T) {
	cache := &tektonCache{
		pipelineRuns: []PipelineRunSnapshot{
			{ManifestID: "m-1", Name: "pr-1", ControlPlaneID: "cp-1", Status: "running"},
			{ManifestID: "m-2", Name: "pr-2", ControlPlaneID: "cp-2", Status: "failed"},
			{ManifestID: "m-1", Name: "pr-3", ControlPlaneID: "cp-1", Status: "succeeded"},
		},
		taskRuns: map[string][]TaskRunSnapshot{
			"m-1": {
				{ManifestID: "m-1", PipelineID: "pr-1", TaskName: "build", TaskRun: "tr-1", Status: "running"},
			},
		},
	}

	snapshot, ok := cache.GetManifestSnapshot("  m-1  ")
	if !ok {
		t.Fatal("expected snapshot to be found")
	}
	if len(snapshot.PipelineRuns) != 2 {
		t.Fatalf("pipeline runs = %d, want 2", len(snapshot.PipelineRuns))
	}
	if len(snapshot.TaskRuns["m-1"]) != 1 {
		t.Fatalf("task runs = %#v, want one copied task run", snapshot.TaskRuns["m-1"])
	}

	snapshot.TaskRuns["m-1"][0].TaskName = "mutated"
	if cache.taskRuns["m-1"][0].TaskName != "build" {
		t.Fatalf("cache task run mutated = %#v", cache.taskRuns["m-1"][0])
	}
}

func TestPollingTektonCacheRefreshPopulatesSnapshots(t *testing.T) {
	now := metav1.NewTime(time.Now().UTC())
	cache := NewPollingTektonCache(
		func(context.Context) ([]tknv1.PipelineRun, error) {
			return []tknv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-1",
						Namespace: "tekton-builds",
						Labels: map[string]string{
							manifestIDLabel:                 "m-1",
							releasedomain.ControlPlaneLabel: "cp-1",
						},
						ResourceVersion: "17",
					},
					Spec: tknv1.PipelineRunSpec{
						PipelineRef: &tknv1.PipelineRef{Name: "build-pipeline"},
					},
					Status: tknv1.PipelineRunStatus{
						Status: duckv1.Status{
							Conditions: duckv1.Conditions{{
								Type:    apis.ConditionSucceeded,
								Status:  corev1.ConditionFalse,
								Message: "build failed",
							}},
						},
						PipelineRunStatusFields: tknv1.PipelineRunStatusFields{
							StartTime:      &now,
							CompletionTime: &now,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ignored",
						Namespace: "tekton-builds",
					},
				},
			}, nil
		},
		func(_ context.Context, namespace, pipelineRunName string) ([]tknv1.TaskRun, error) {
			if namespace != "tekton-builds" {
				t.Fatalf("namespace = %q, want tekton-builds", namespace)
			}
			if pipelineRunName != "pr-1" {
				t.Fatalf("pipelineRunName = %q, want pr-1", pipelineRunName)
			}
			return []tknv1.TaskRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tr-1",
						Labels: map[string]string{
							pipelineTaskLabel: "build-image",
						},
					},
					Spec: tknv1.TaskRunSpec{
						Params: []tknv1.Param{
							{Name: "IMAGE_REPOSITORY", Value: *tknv1.NewStructuredValues("registry.example.com/devflow/demo")},
							{Name: "IMAGE_TAG", Value: *tknv1.NewStructuredValues("20260428")},
						},
					},
					Status: tknv1.TaskRunStatus{
						Status: duckv1.Status{
							Conditions: duckv1.Conditions{{
								Type:    apis.ConditionSucceeded,
								Status:  corev1.ConditionUnknown,
								Message: "still running",
							}},
						},
						TaskRunStatusFields: tknv1.TaskRunStatusFields{
							StartTime: &now,
							Results: []tknv1.TaskRunResult{
								{Name: "commit", Value: *tknv1.NewStructuredValues("abc123")},
								{Name: "IMAGE_TAG", Value: *tknv1.NewStructuredValues("20260428")},
								{Name: "IMAGE_DIGEST", Value: *tknv1.NewStructuredValues("sha256:abc")},
							},
						},
					},
				},
			}, nil
		},
		"build-pipeline",
	)

	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if got := cache.ListManifestIDs("cp-1"); len(got) != 1 || got[0] != "m-1" {
		t.Fatalf("ListManifestIDs(cp-1) = %#v, want [m-1]", got)
	}

	snapshot, ok := cache.GetManifestSnapshot("m-1")
	if !ok {
		t.Fatal("expected snapshot for manifest m-1")
	}
	if len(snapshot.PipelineRuns) != 1 {
		t.Fatalf("pipeline runs = %d, want 1", len(snapshot.PipelineRuns))
	}
	pipeline := snapshot.PipelineRuns[0]
	if pipeline.Name != "pr-1" || pipeline.Namespace != "tekton-builds" || pipeline.ManifestID != "m-1" || pipeline.ControlPlaneID != "cp-1" {
		t.Fatalf("pipeline snapshot = %#v", pipeline)
	}
	if pipeline.Status != string(releasedomain.ManifestUnavailable) || pipeline.Message != "build failed" || pipeline.StateKey == "" {
		t.Fatalf("pipeline status snapshot = %#v", pipeline)
	}
	taskRuns := snapshot.TaskRuns["m-1"]
	if len(taskRuns) != 1 {
		t.Fatalf("task runs = %#v, want one task snapshot", taskRuns)
	}
	task := taskRuns[0]
	if task.ManifestID != "m-1" || task.PipelineID != "pr-1" || task.TaskName != "build-image" || task.TaskRun != "tr-1" {
		t.Fatalf("task snapshot = %#v", task)
	}
	if task.Status != string(releasedomain.StepRunning) || task.Message != "still running" {
		t.Fatalf("task status snapshot = %#v", task)
	}
	if task.Params["IMAGE_REPOSITORY"] != "registry.example.com/devflow/demo" {
		t.Fatalf("task params = %#v", task.Params)
	}
	if task.Results["IMAGE_DIGEST"] != "sha256:abc" || task.Results["commit"] != "abc123" {
		t.Fatalf("task results = %#v", task.Results)
	}
}

func TestPollingTektonCacheRefreshKeepsLastGoodSnapshotOnError(t *testing.T) {
	var fail bool
	cache := NewPollingTektonCache(
		func(context.Context) ([]tknv1.PipelineRun, error) {
			if fail {
				return nil, errors.New("boom")
			}
			return []tknv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-1",
						Namespace: "tekton-builds",
						Labels: map[string]string{
							manifestIDLabel: "m-1",
						},
					},
					Spec: tknv1.PipelineRunSpec{
						PipelineRef: &tknv1.PipelineRef{Name: "build-pipeline"},
					},
				},
			}, nil
		},
		func(context.Context, string, string) ([]tknv1.TaskRun, error) {
			return nil, nil
		},
		"build-pipeline",
	)

	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("first Refresh() error = %v", err)
	}
	fail = true
	if err := cache.Refresh(context.Background()); err == nil {
		t.Fatal("expected second Refresh() to fail")
	}

	if got := cache.ListManifestIDs(""); len(got) != 1 || got[0] != "m-1" {
		t.Fatalf("ListManifestIDs() after failed refresh = %#v, want [m-1]", got)
	}
}

func TestPollingTektonCacheRefreshFiltersByPipelineName(t *testing.T) {
	cache := NewPollingTektonCache(
		func(context.Context) ([]tknv1.PipelineRun, error) {
			return []tknv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-1",
						Namespace: "tekton-builds",
						Labels: map[string]string{
							manifestIDLabel: "m-1",
						},
					},
					Spec: tknv1.PipelineRunSpec{
						PipelineRef: &tknv1.PipelineRef{Name: "build-pipeline"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-2",
						Namespace: "tekton-builds",
						Labels: map[string]string{
							manifestIDLabel: "m-2",
						},
					},
					Spec: tknv1.PipelineRunSpec{
						PipelineRef: &tknv1.PipelineRef{Name: "other-pipeline"},
					},
				},
			}, nil
		},
		func(context.Context, string, string) ([]tknv1.TaskRun, error) {
			return nil, nil
		},
		"build-pipeline",
	)

	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if got := cache.ListManifestIDs(""); len(got) != 1 || got[0] != "m-1" {
		t.Fatalf("ListManifestIDs() = %#v, want [m-1]", got)
	}
}

func TestBuildManifestResultSnapshot(t *testing.T) {
	result := BuildManifestResultSnapshot([]TaskRunSnapshot{
		{
			TaskName: "git-clone",
			Results: map[string]string{
				"commit": "abc123",
			},
		},
		{
			TaskName: "build-image",
			Params: map[string]string{
				"IMAGE_REPOSITORY": "registry.example.com/devflow/demo",
				"IMAGE_TAG":        "20260428",
			},
			Results: map[string]string{
				"IMAGE_TAG":    "20260428",
				"IMAGE_DIGEST": "sha256:abc",
			},
		},
	})
	if result == nil {
		t.Fatal("expected result snapshot")
	}
	if result.CommitHash != "abc123" || result.ImageTag != "20260428" || result.ImageDigest != "sha256:abc" {
		t.Fatalf("result = %#v", result)
	}
	if result.ImageRef != "registry.example.com/devflow/demo@sha256:abc" {
		t.Fatalf("image ref = %q", result.ImageRef)
	}
}
