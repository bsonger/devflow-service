package watch

import (
	"context"
	"reflect"
	"testing"

	"github.com/bsonger/devflow-service/internal/platform/observer"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestManifestEventSourceEnqueuesManifestOnPipelineRunUpdate(t *testing.T) {
	cache := newInformerTektonCacheForTest("build-pipeline")
	queue := &recordingManifestQueueForEventSource{}
	source := NewManifestEventSource(cache, queue, "cp-1")

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
	err := source.handleObject(&tknv1.PipelineRun{
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
	if err != nil {
		t.Fatalf("handleObject(pipelineRun) error = %v", err)
	}

	if !reflect.DeepEqual(queue.added, []string{"m-1"}) {
		t.Fatalf("queued manifests = %#v, want [m-1]", queue.added)
	}
}

func TestManifestEventSourceEnqueuesManifestOnTaskRunUpdate(t *testing.T) {
	cache := newInformerTektonCacheForTest("build-pipeline")
	queue := &recordingManifestQueueForEventSource{}
	source := NewManifestEventSource(cache, queue, "cp-1")

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
	cache.upsertTaskRun(&tknv1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr-1",
			Namespace: "tekton-pipelines",
			Labels: map[string]string{
				"tekton.dev/pipelineRun": "pr-1",
				pipelineTaskLabel:        "build-image",
			},
		},
	})
	err := source.handleObject(&tknv1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr-1",
			Namespace: "tekton-pipelines",
			Labels: map[string]string{
				"tekton.dev/pipelineRun": "pr-1",
				pipelineTaskLabel:        "build-image",
			},
		},
	})
	if err != nil {
		t.Fatalf("handleObject(taskRun) error = %v", err)
	}

	if !reflect.DeepEqual(queue.added, []string{"m-1"}) {
		t.Fatalf("queued manifests = %#v, want [m-1]", queue.added)
	}
}

func TestManifestEventSourceSkipsDifferentControlPlane(t *testing.T) {
	cache := newInformerTektonCacheForTest("build-pipeline")
	queue := &recordingManifestQueueForEventSource{}
	source := NewManifestEventSource(cache, queue, "cp-1")

	err := source.handleObject(&tknv1.PipelineRun{
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
	if err != nil {
		t.Fatalf("handleObject error = %v", err)
	}

	if len(queue.added) != 0 {
		t.Fatalf("queued manifests = %#v, want none", queue.added)
	}
}

func TestManifestEventSourceSkipsDonePipelineRun(t *testing.T) {
	cache := newInformerTektonCacheForTest("build-pipeline")
	queue := &recordingManifestQueueForEventSource{}
	source := NewManifestEventSource(cache, queue, "cp-1")

	err := source.handleObject(&tknv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pr-3",
			Namespace: "tekton-pipelines",
			Labels: map[string]string{
				manifestIDLabel:                 "m-3",
				releasedomain.ControlPlaneLabel: "cp-1",
				observer.ObserveStateLabel:      observer.ObserveStateDone,
			},
		},
		Spec: tknv1.PipelineRunSpec{
			PipelineRef: &tknv1.PipelineRef{Name: "build-pipeline"},
		},
	})
	if err != nil {
		t.Fatalf("handleObject error = %v", err)
	}

	if len(queue.added) != 0 {
		t.Fatalf("queued manifests = %#v, want none", queue.added)
	}
}

type recordingManifestQueueForEventSource struct {
	added []string
}

func (q *recordingManifestQueueForEventSource) Add(manifestID string) {
	q.added = append(q.added, manifestID)
}

func (q *recordingManifestQueueForEventSource) Run(_ context.Context, _ int, _ func(context.Context, string) error) {
}

func (q *recordingManifestQueueForEventSource) ShutDown() {}
