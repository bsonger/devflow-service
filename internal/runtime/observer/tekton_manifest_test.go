package observer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/observer"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestBuildResultPayload(t *testing.T) {
	taskRuns := []tknv1.TaskRun{
		{
			Status: tknv1.TaskRunStatus{
				TaskRunStatusFields: tknv1.TaskRunStatusFields{
					Results: []tknv1.TaskRunResult{{Name: "commit", Value: *tknv1.NewStructuredValues("abc123")}},
				},
			},
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"tekton.dev/pipelineTask": "git-clone"}},
		},
		{
			Spec: tknv1.TaskRunSpec{
				Params: []tknv1.Param{
					{Name: "IMAGE_REPOSITORY", Value: *tknv1.NewStructuredValues("registry.example.com/devflow/demo")},
					{Name: "IMAGE_TAG", Value: *tknv1.NewStructuredValues("20260428")},
				},
			},
			Status: tknv1.TaskRunStatus{
				TaskRunStatusFields: tknv1.TaskRunStatusFields{
					Results: []tknv1.TaskRunResult{
						{Name: "IMAGE_TAG", Value: *tknv1.NewStructuredValues("20260428")},
						{Name: "IMAGE_DIGEST", Value: *tknv1.NewStructuredValues("sha256:abc")},
					},
				},
			},
		},
	}
	got := buildResultPayload("manifest-1", "pipe-1", taskRuns)
	if got["commit_hash"] != "abc123" {
		t.Fatalf("commit_hash = %v", got["commit_hash"])
	}
	if got["image_ref"] != "registry.example.com/devflow/demo@sha256:abc" {
		t.Fatalf("image_ref = %v", got["image_ref"])
	}
	if got["image_tag"] != "20260428" {
		t.Fatalf("image_tag = %v", got["image_tag"])
	}
	if got["image_digest"] != "sha256:abc" {
		t.Fatalf("image_digest = %v", got["image_digest"])
	}
}

func TestTektonObserverSelectorUsesObserveStateRunning(t *testing.T) {
	if got := manifestIDLabel + "," + observer.ObserveStateLabel + "=" + observer.ObserveStateRunning; got != "devflow.manifest/id,devflow.io/observe-state=running" {
		t.Fatalf("selector = %q", got)
	}
}

func TestMapPipelineAndTaskStatuses(t *testing.T) {
	now := metav1.NewTime(time.Now())
	pr := &tknv1.PipelineRun{
		Status: tknv1.PipelineRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionTrue,
					Message: "done",
				}},
			},
			PipelineRunStatusFields: tknv1.PipelineRunStatusFields{
				StartTime: &now,
			},
		},
	}
	if got := mapPipelineRunStatus(pr); got != model.ManifestAvailable {
		t.Fatalf("pipeline status = %s", got)
	}
	tr := &tknv1.TaskRun{
		Status: tknv1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
				}},
			},
			TaskRunStatusFields: tknv1.TaskRunStatusFields{
				StartTime: &now,
			},
		},
	}
	if got := mapTaskRunStatus(tr); got != model.StepFailed {
		t.Fatalf("task status = %s", got)
	}
}

func TestSyncPipelineRunRetriesAfterNotFoundWriteback(t *testing.T) {
	var mode atomic.Int32
	mode.Store(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mode.Load() == 0 {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	now := metav1.NewTime(time.Now())
	pr := &tknv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipe-1",
			Namespace: "tekton-pipelines",
			Labels:    map[string]string{manifestIDLabel: "manifest-1"},
		},
		Status: tknv1.PipelineRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionTrue,
					Message: "done",
				}},
			},
			PipelineRunStatusFields: tknv1.PipelineRunStatusFields{
				StartTime:      &now,
				CompletionTime: &now,
			},
		},
	}
	taskRun := &tknv1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipe-1-git-clone",
			Namespace: "tekton-pipelines",
			Labels: map[string]string{
				"tekton.dev/pipelineRun":  "pipe-1",
				"tekton.dev/pipelineTask": "git-clone",
			},
		},
		Status: tknv1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}},
			},
			TaskRunStatusFields: tknv1.TaskRunStatusFields{
				StartTime:      &now,
				CompletionTime: &now,
				Results: []tknv1.TaskRunResult{
					{Name: "commit", Value: *tknv1.NewStructuredValues("abc123")},
				},
			},
		},
	}

	observer := &TektonManifestObserver{
		cfg:    TektonManifestObserverConfig{ReleaseServiceBaseURL: server.URL},
		tekton: tektonfake.NewSimpleClientset(taskRun),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		releaseBase: server.URL,
		processed:   map[string]string{},
	}

	if err := observer.syncPipelineRun(context.Background(), pr); err != nil {
		t.Fatalf("syncPipelineRun() first call error = %v", err)
	}
	if observer.isProcessed(pr.Name, pipelineRunStateKey(pr)) {
		t.Fatal("terminal pipeline was marked processed after 404 writeback")
	}

	mode.Store(1)
	if err := observer.syncPipelineRun(context.Background(), pr); err != nil {
		t.Fatalf("syncPipelineRun() second call error = %v", err)
	}
	if !observer.isProcessed(pr.Name, pipelineRunStateKey(pr)) {
		t.Fatal("terminal pipeline was not marked processed after successful writeback")
	}
}
