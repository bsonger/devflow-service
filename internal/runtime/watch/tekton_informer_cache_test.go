package watch

import (
	"reflect"
	"testing"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestInformerTektonCacheListsManifestIDsByControlPlane(t *testing.T) {
	cache := newInformerTektonCacheForTest("build-pipeline")
	cache.upsertPipelineRun(&tknv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pr-1",
			Namespace: "tekton-pipelines",
			Labels: map[string]string{
				manifestIDLabel:                 "m-1",
				releasedomain.ControlPlaneLabel: "cp-1",
			},
		},
		Spec: tknv1.PipelineRunSpec{
			PipelineRef: &tknv1.PipelineRef{Name: "build-pipeline"},
		},
	})
	cache.upsertPipelineRun(&tknv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pr-2",
			Namespace: "tekton-pipelines",
			Labels: map[string]string{
				manifestIDLabel:                 "m-2",
				releasedomain.ControlPlaneLabel: "cp-2",
			},
		},
		Spec: tknv1.PipelineRunSpec{
			PipelineRef: &tknv1.PipelineRef{Name: "build-pipeline"},
		},
	})

	got := cache.ListManifestIDs("cp-1")
	if !reflect.DeepEqual(got, []string{"m-1"}) {
		t.Fatalf("ListManifestIDs(cp-1) = %#v, want [m-1]", got)
	}
}

func TestInformerTektonCacheBuildsSnapshotWithTaskResults(t *testing.T) {
	now := metav1.NewTime(time.Now().UTC())
	cache := newInformerTektonCacheForTest("build-pipeline")
	cache.upsertPipelineRun(&tknv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pr-1",
			Namespace: "tekton-pipelines",
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
					Status:  corev1.ConditionTrue,
					Message: "pipeline done",
				}},
			},
			PipelineRunStatusFields: tknv1.PipelineRunStatusFields{
				StartTime:      &now,
				CompletionTime: &now,
			},
		},
	})
	cache.upsertTaskRun(&tknv1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr-1",
			Namespace: "tekton-pipelines",
			Labels: map[string]string{
				"tekton.dev/pipelineRun": "pr-1",
				pipelineTaskLabel:        "build-image",
			},
		},
		Spec: tknv1.TaskRunSpec{
			Params: []tknv1.Param{
				{Name: "IMAGE_REPOSITORY", Value: *tknv1.NewStructuredValues("registry.example.com/devflow/demo")},
				{Name: "IMAGE_TAG", Value: *tknv1.NewStructuredValues("20260517")},
			},
		},
		Status: tknv1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionTrue,
					Message: "image pushed",
				}},
			},
			TaskRunStatusFields: tknv1.TaskRunStatusFields{
				StartTime: &now,
				Results: []tknv1.TaskRunResult{
					{Name: "commit", Value: *tknv1.NewStructuredValues("abc123")},
					{Name: "IMAGE_TAG", Value: *tknv1.NewStructuredValues("20260517")},
					{Name: "IMAGE_DIGEST", Value: *tknv1.NewStructuredValues("sha256:def")},
				},
			},
		},
	})

	snapshot, ok := cache.GetManifestSnapshot("m-1")
	if !ok {
		t.Fatal("expected snapshot for m-1")
	}
	if len(snapshot.PipelineRuns) != 1 {
		t.Fatalf("pipeline runs = %d, want 1", len(snapshot.PipelineRuns))
	}
	if len(snapshot.TaskRuns["m-1"]) != 1 {
		t.Fatalf("task runs = %#v, want 1 task", snapshot.TaskRuns["m-1"])
	}
	task := snapshot.TaskRuns["m-1"][0]
	if task.TaskName != "build-image" || task.TaskRun != "tr-1" || task.Message != "image pushed" {
		t.Fatalf("task snapshot = %#v", task)
	}
	result := BuildManifestResultSnapshot(snapshot.TaskRuns["m-1"])
	if result == nil {
		t.Fatal("expected result snapshot")
	}
	if result.CommitHash != "abc123" || result.ImageTag != "20260517" || result.ImageDigest != "sha256:def" || result.ImageRef != "registry.example.com/devflow/demo@sha256:def" {
		t.Fatalf("result snapshot = %#v", result)
	}
}
