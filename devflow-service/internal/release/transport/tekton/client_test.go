package tekton

import (
	"testing"

	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestWithPVCOwnerSetsPipelineRunOwnerReference(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-1",
			Namespace: "tekton-pipelines",
		},
	}
	run := &tknv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "run-1",
			Namespace: "tekton-pipelines",
			UID:       types.UID("uid-1"),
		},
	}

	updated := withPVCOwner(pvc, run)
	if len(updated.OwnerReferences) != 1 {
		t.Fatalf("owner refs = %d want 1", len(updated.OwnerReferences))
	}
	if updated.OwnerReferences[0].Name != "run-1" {
		t.Fatalf("owner ref name = %q want run-1", updated.OwnerReferences[0].Name)
	}
	if updated.OwnerReferences[0].UID != types.UID("uid-1") {
		t.Fatalf("owner ref uid = %q want uid-1", updated.OwnerReferences[0].UID)
	}
}
