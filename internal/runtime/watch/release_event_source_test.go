package watch

import (
	"context"
	"reflect"
	"testing"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

func TestReleaseEventSourceEnqueuesReleaseOwnedDeployment(t *testing.T) {
	queue := &recordingReleaseQueueForEventSource{}
	source := NewReleaseEventSource(nil, queue, "cp-1")

	source.handleObject(&appsv1.Deployment{})
	source.handleObject(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
			Labels: map[string]string{
				releasedomain.ReleaseIDLabel:    "11111111-1111-1111-1111-111111111111",
				releasedomain.ControlPlaneLabel: "cp-1",
				releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
			},
		},
	})

	if !reflect.DeepEqual(queue.added, []string{"11111111-1111-1111-1111-111111111111"}) {
		t.Fatalf("queued release ids = %#v", queue.added)
	}
}

func TestReleaseEventSourceSkipsDifferentControlPlane(t *testing.T) {
	queue := &recordingReleaseQueueForEventSource{}
	source := NewReleaseEventSource(nil, queue, "cp-1")

	source.handleObject(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-pod",
			Namespace: "default",
			Labels: map[string]string{
				releasedomain.ReleaseIDLabel:    "22222222-2222-2222-2222-222222222222",
				releasedomain.ControlPlaneLabel: "cp-2",
				releasedomain.ReleaseStatusLabel: string(releasedomain.ReleaseRunning),
			},
		},
	})

	if len(queue.added) != 0 {
		t.Fatalf("queued release ids = %#v, want none", queue.added)
	}
}

func TestReleaseEventSourceSkipsNonRunningReleaseStatus(t *testing.T) {
	queue := &recordingReleaseQueueForEventSource{}
	source := NewReleaseEventSource(nil, queue, "cp-1")

	source.handleObject(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				releasedomain.ReleaseIDLabel:     "11111111-1111-1111-1111-111111111111",
				releasedomain.ControlPlaneLabel:  "cp-1",
				releasedomain.ReleaseStatusLabel: "succeeded",
			},
		},
	})

	if len(queue.added) != 0 {
		t.Fatalf("queued release ids = %#v, want none", queue.added)
	}
}

func TestReleaseLabelsFromObjectSupportsRolloutDeleteTombstone(t *testing.T) {
	labels := releaseLabelsFromObject(cache.DeletedFinalStateUnknown{
		Obj: &unstructured.Unstructured{Object: map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					releasedomain.ReleaseIDLabel: "33333333-3333-3333-3333-333333333333",
				},
			},
		}},
	})
	if labels[releasedomain.ReleaseIDLabel] != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("labels = %#v", labels)
	}
}

type recordingReleaseQueueForEventSource struct {
	added []string
}

func (q *recordingReleaseQueueForEventSource) Add(releaseID string) {
	q.added = append(q.added, releaseID)
}

func (q *recordingReleaseQueueForEventSource) Run(_ context.Context, _ int, _ func(context.Context, string) error) {
}

func (q *recordingReleaseQueueForEventSource) ShutDown() {}
