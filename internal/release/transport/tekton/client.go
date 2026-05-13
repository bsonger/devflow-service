package tekton

import (
	"context"
	"encoding/json"

	"github.com/bsonger/devflow-service/internal/platform/observer"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var tektonClient *tektonclient.Clientset
var kubeClient *kubernetes.Clientset

func InitClient(ctx context.Context, config *rest.Config, logger *zap.Logger) error {
	var err error
	tektonClient, err = tektonclient.NewForConfig(config)
	if err != nil {
		return err
	}
	kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	logger.Info("tekton client initialized",
		zap.String("operation", "init_tekton_client"),
		zap.String("resource", "tekton_client"),
		zap.String("result", "success"),
	)
	return nil
}

func GetPipeline(ctx context.Context, namespace string, name string) (*tknv1.Pipeline, error) {
	var pipeline *tknv1.Pipeline
	err := platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "tekton",
		Operation: "get_pipeline",
	}, func(depCtx context.Context) error {
		var err error
		pipeline, err = tektonClient.TektonV1().Pipelines(namespace).Get(depCtx, name, metav1.GetOptions{})
		return err
	})
	return pipeline, err
}

func CreatePipelineRun(ctx context.Context, namespace string, pr *tknv1.PipelineRun) (*tknv1.PipelineRun, error) {
	var pipelineRun *tknv1.PipelineRun
	err := platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "tekton",
		Operation: "create_pipeline_run",
	}, func(depCtx context.Context) error {
		var err error
		pipelineRun, err = tektonClient.TektonV1().PipelineRuns(namespace).Create(depCtx, pr, metav1.CreateOptions{})
		return err
	})
	return pipelineRun, err
}

func MarkPipelineRunObserveState(ctx context.Context, namespace, name, state string) error {
	return platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "tekton",
		Operation: "mark_pipeline_run_observe_state",
	}, func(depCtx context.Context) error {
		run, err := tektonClient.TektonV1().PipelineRuns(namespace).Get(depCtx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		copy := run.DeepCopy()
		if copy.Labels == nil {
			copy.Labels = map[string]string{}
		}
		copy.Labels[observer.ObserveStateLabel] = state
		_, err = tektonClient.TektonV1().PipelineRuns(namespace).Update(depCtx, copy, metav1.UpdateOptions{})
		return err
	})
}

func CreatePVC(ctx context.Context, namespace, pvcName, storageClassName string, size string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: pvcName + "-",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
			StorageClassName: &storageClassName,
		},
	}

	var created *corev1.PersistentVolumeClaim
	err := platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "kubernetes",
		Operation: "create_persistent_volume_claim",
	}, func(depCtx context.Context) error {
		var err error
		created, err = kubeClient.CoreV1().PersistentVolumeClaims(namespace).Create(depCtx, pvc, metav1.CreateOptions{})
		return err
	})
	return created, err
}

func PatchPVCOwner(ctx context.Context, pvc *corev1.PersistentVolumeClaim, pr *tknv1.PipelineRun) error {
	oldData, err := json.Marshal(pvc)
	if err != nil {
		return err
	}

	updated := withPVCOwner(pvc, pr)
	newData, err := json.Marshal(updated)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, pvc)
	if err != nil {
		return err
	}

	return platformobs.ObserveDependency(ctx, platformobs.DependencyCall{
		Kind:      "k8s",
		Target:    "kubernetes",
		Operation: "patch_persistent_volume_claim_owner",
	}, func(depCtx context.Context) error {
		_, err := kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Patch(depCtx, pvc.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		return err
	})
}

func withPVCOwner(pvc *corev1.PersistentVolumeClaim, pr *tknv1.PipelineRun) *corev1.PersistentVolumeClaim {
	copy := pvc.DeepCopy()
	copy.OwnerReferences = append(copy.OwnerReferences, *metav1.NewControllerRef(
		pr,
		tknv1.SchemeGroupVersion.WithKind("PipelineRun"),
	))
	return copy
}
