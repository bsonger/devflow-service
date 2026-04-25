package tekton

import (
	"context"
	"encoding/json"

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
	return tektonClient.TektonV1().Pipelines(namespace).Get(ctx, name, metav1.GetOptions{})
}

func CreatePipelineRun(ctx context.Context, namespace string, pr *tknv1.PipelineRun) (*tknv1.PipelineRun, error) {
	return tektonClient.TektonV1().PipelineRuns(namespace).Create(ctx, pr, metav1.CreateOptions{})
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

	return kubeClient.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
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

	_, err = kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Patch(ctx, pvc.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	return err
}

func withPVCOwner(pvc *corev1.PersistentVolumeClaim, pr *tknv1.PipelineRun) *corev1.PersistentVolumeClaim {
	copy := pvc.DeepCopy()
	copy.OwnerReferences = append(copy.OwnerReferences, *metav1.NewControllerRef(
		pr,
		tknv1.SchemeGroupVersion.WithKind("PipelineRun"),
	))
	return copy
}
