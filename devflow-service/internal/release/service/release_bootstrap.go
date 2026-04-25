package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/internal/release/transport/argo"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	ErrBootstrapNamespace         = errors.New("bootstrap namespace failed")
	ErrBootstrapPullSecret        = errors.New("bootstrap pull secret failed")
	ErrBootstrapAppProject        = errors.New("bootstrap app project destination failed")
	ErrBootstrapMissingTarget     = errors.New("bootstrap missing deploy target")
	ErrBootstrapMissingKubeConfig = errors.New("bootstrap missing kubernetes config")
)

// bootstrapExecutor runs ordered bootstrap gates before Argo Application apply.
type bootstrapExecutor struct {
	kubeClient kubernetes.Interface
}

// newBootstrapExecutor creates a bootstrap executor using the shared KubeConfig.
func newBootstrapExecutor() (*bootstrapExecutor, error) {
	if model.KubeConfig == nil {
		return nil, ErrBootstrapMissingKubeConfig
	}
	client, err := kubernetes.NewForConfig(model.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	return &bootstrapExecutor{kubeClient: client}, nil
}

// bootstrapResult captures the outcome of a single gate.
type bootstrapResult struct {
	StepName string
	Status   model.StepStatus
	Message  string
	Start    *time.Time
	End      *time.Time
}

// runBootstrapGates executes bootstrap gates in order and returns on first failure.
// The caller is responsible for persisting step outcomes via UpdateStep.
func (e *bootstrapExecutor) runBootstrapGates(ctx context.Context, target deployTarget, appProjectName string) ([]bootstrapResult, error) {
	log := logger.LoggerWithContext(ctx)
	if log == nil {
		log = zap.NewNop()
	}

	if target.Namespace == "" || target.DestinationServer == "" {
		return nil, fmt.Errorf("%w: namespace=%q server=%q", ErrBootstrapMissingTarget, target.Namespace, target.DestinationServer)
	}

	results := make([]bootstrapResult, 0, 3)

	// Gate 1: ensure namespace exists
	res := e.gateEnsureNamespace(ctx, target.Namespace)
	results = append(results, res)
	if res.Status == model.StepFailed {
		log.Error("bootstrap gate failed", zap.String("gate", res.StepName), zap.String("message", res.Message))
		return results, fmt.Errorf("%w: %s", ErrBootstrapNamespace, res.Message)
	}

	// Gate 2: ensure image pull secret exists in target namespace
	res = e.gateEnsurePullSecret(ctx, target.Namespace)
	results = append(results, res)
	if res.Status == model.StepFailed {
		log.Error("bootstrap gate failed", zap.String("gate", res.StepName), zap.String("message", res.Message))
		return results, fmt.Errorf("%w: %s", ErrBootstrapPullSecret, res.Message)
	}

	// Gate 3: ensure AppProject destination allowlist contains target server+namespace
	res = e.gateEnsureAppProjectDestination(ctx, appProjectName, target.DestinationServer, target.Namespace)
	results = append(results, res)
	if res.Status == model.StepFailed {
		log.Error("bootstrap gate failed", zap.String("gate", res.StepName), zap.String("message", res.Message))
		return results, fmt.Errorf("%w: %s", ErrBootstrapAppProject, res.Message)
	}

	return results, nil
}

func (e *bootstrapExecutor) gateEnsureNamespace(ctx context.Context, namespace string) bootstrapResult {
	now := time.Now()
	res := bootstrapResult{
		StepName: "ensure namespace",
		Status:   model.StepRunning,
		Start:    &now,
	}

	_, err := e.kubeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		end := time.Now()
		res.Status = model.StepSucceeded
		res.Message = "namespace already exists"
		res.End = &end
		return res
	}
	if !apierrors.IsNotFound(err) {
		end := time.Now()
		res.Status = model.StepFailed
		res.Message = fmt.Sprintf("namespace lookup failed: %v", err)
		res.End = &end
		return res
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"devflow.io/managed-by": "devflow-release-service",
			},
		},
	}
	_, err = e.kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		end := time.Now()
		res.Status = model.StepFailed
		res.Message = fmt.Sprintf("namespace create failed: %v", err)
		res.End = &end
		return res
	}

	end := time.Now()
	res.Status = model.StepSucceeded
	res.Message = "namespace created"
	res.End = &end
	return res
}

func (e *bootstrapExecutor) gateEnsurePullSecret(ctx context.Context, namespace string) bootstrapResult {
	now := time.Now()
	res := bootstrapResult{
		StepName: "ensure pull secret",
		Status:   model.StepRunning,
		Start:    &now,
	}

	// Look for an existing pull secret in the release-service namespace to copy from.
	// If none exists, treat as idempotent success (cluster may use other auth).
	sourceNS := "devflow-release-service"
	sourceName := "registry-credentials"

	source, err := e.kubeClient.CoreV1().Secrets(sourceNS).Get(ctx, sourceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			end := time.Now()
			res.Status = model.StepSucceeded
			res.Message = "no source pull secret to copy; skipped"
			res.End = &end
			return res
		}
		end := time.Now()
		res.Status = model.StepFailed
		res.Message = fmt.Sprintf("source pull secret lookup failed: %v", err)
		res.End = &end
		return res
	}

	_, err = e.kubeClient.CoreV1().Secrets(namespace).Get(ctx, sourceName, metav1.GetOptions{})
	if err == nil {
		end := time.Now()
		res.Status = model.StepSucceeded
		res.Message = "pull secret already exists in target namespace"
		res.End = &end
		return res
	}
	if !apierrors.IsNotFound(err) {
		end := time.Now()
		res.Status = model.StepFailed
		res.Message = fmt.Sprintf("target pull secret lookup failed: %v", err)
		res.End = &end
		return res
	}

	// Copy the source secret into the target namespace
	secretCopy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourceName,
			Namespace: namespace,
			Labels: map[string]string{
				"devflow.io/managed-by": "devflow-release-service",
			},
		},
		Type: source.Type,
		Data: source.Data,
	}
	_, err = e.kubeClient.CoreV1().Secrets(namespace).Create(ctx, secretCopy, metav1.CreateOptions{})
	if err != nil {
		end := time.Now()
		res.Status = model.StepFailed
		res.Message = fmt.Sprintf("pull secret create failed: %v", err)
		res.End = &end
		return res
	}

	end := time.Now()
	res.Status = model.StepSucceeded
	res.Message = "pull secret copied to target namespace"
	res.End = &end
	return res
}

func (e *bootstrapExecutor) gateEnsureAppProjectDestination(ctx context.Context, appProjectName, server, namespace string) bootstrapResult {
	now := time.Now()
	res := bootstrapResult{
		StepName: "ensure appproject destination",
		Status:   model.StepRunning,
		Start:    &now,
	}

	if appProjectName == "" {
		appProjectName = "app"
	}

	project, err := argoclient.GetAppProject(ctx, appProjectName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			end := time.Now()
			res.Status = model.StepFailed
			res.Message = fmt.Sprintf("appproject %q not found", appProjectName)
			res.End = &end
			return res
		}
		end := time.Now()
		res.Status = model.StepFailed
		res.Message = fmt.Sprintf("appproject lookup failed: %v", err)
		res.End = &end
		return res
	}

	// Check if destination already present
	for _, dest := range project.Spec.Destinations {
		if dest.Server == server && dest.Namespace == namespace {
			end := time.Now()
			res.Status = model.StepSucceeded
			res.Message = "destination already in allowlist"
			res.End = &end
			return res
		}
	}

	// Add destination to allowlist
	project.Spec.Destinations = append(project.Spec.Destinations, appv1.ApplicationDestination{
		Server:    server,
		Namespace: namespace,
	})

	if err := argoclient.UpdateAppProject(ctx, project); err != nil {
		end := time.Now()
		res.Status = model.StepFailed
		res.Message = fmt.Sprintf("appproject update failed: %v", err)
		res.End = &end
		return res
	}

	end := time.Now()
	res.Status = model.StepSucceeded
	res.Message = "destination added to appproject allowlist"
	res.End = &end
	return res
}
