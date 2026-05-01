package argoclient

import (
	"context"
	"fmt"
	"strings"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argoapi "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1"
	"github.com/bsonger/devflow-service/internal/platform/logger"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

var Client ArgoClientInterface

const namespace = "argocd"
const restartedAtIgnorePointer = "/spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt"

type ArgoClientInterface interface {
	ArgoprojV1alpha1() argov1alpha1.ArgoprojV1alpha1Interface
}

type applicationAPI interface {
	Create(ctx context.Context, app *appv1.Application, opts metav1.CreateOptions) (*appv1.Application, error)
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*appv1.Application, error)
	Update(ctx context.Context, app *appv1.Application, opts metav1.UpdateOptions) (*appv1.Application, error)
}

type ApplicationInspection struct {
	Name                        string
	Namespace                   string
	Project                     string
	SyncStatus                  string
	HealthStatus                string
	OperationPhase              string
	OperationMessage            string
	ReconciledAt                string
	Revision                    string
	TargetRevision              string
	RepoURL                     string
	DestinationServer           string
	DestinationNamespace        string
	IgnoreDifferenceTargets     []IgnoreDifferenceTarget
	OutOfSyncResources          []ApplicationResourceObservation
	RestartedAtCandidates       []ApplicationResourceObservation
	RestartedAtIgnoreConfigured bool
}

type IgnoreDifferenceTarget struct {
	Group        string
	Kind         string
	Name         string
	Namespace    string
	JSONPointers []string
}

type ApplicationResourceObservation struct {
	Group           string
	Kind            string
	Namespace       string
	Name            string
	SyncStatus      string
	HealthStatus    string
	RequiresPruning bool
	SyncWave        int64
}

func Init(config *rest.Config) error {
	var err error
	Client, err = argoapi.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create argo cd client: %w", err)
	}
	logger.Logger.Info("argo cd client initialized",
		zap.String("operation", "init_argo_client"),
		zap.String("resource", "argo_client"),
		zap.String("result", "success"),
	)
	return nil
}

func CreateApplication(ctx context.Context, app *appv1.Application) error {
	_, err := Client.ArgoprojV1alpha1().Applications(namespace).Create(ctx, app, metav1.CreateOptions{})
	return err
}

func UpdateApplication(ctx context.Context, app *appv1.Application) error {
	return applyApplication(ctx, Client.ArgoprojV1alpha1().Applications(namespace), app)
}

func GetApplication(ctx context.Context, name string) (*appv1.Application, error) {
	return Client.ArgoprojV1alpha1().Applications(namespace).Get(ctx, name, metav1.GetOptions{})
}

func InspectApplication(ctx context.Context, name string) (*ApplicationInspection, error) {
	app, err := GetApplication(ctx, name)
	if err != nil {
		return nil, err
	}
	return BuildApplicationInspection(app), nil
}

func BuildApplicationInspection(app *appv1.Application) *ApplicationInspection {
	if app == nil {
		return nil
	}
	inspection := &ApplicationInspection{
		Name:                 strings.TrimSpace(app.Name),
		Namespace:            strings.TrimSpace(app.Namespace),
		Project:              strings.TrimSpace(app.Spec.Project),
		SyncStatus:           strings.TrimSpace(string(app.Status.Sync.Status)),
		HealthStatus:         strings.TrimSpace(string(app.Status.Health.Status)),
		Revision:             strings.TrimSpace(app.Status.Sync.Revision),
		RepoURL:              strings.TrimSpace(app.Spec.GetSource().RepoURL),
		TargetRevision:       strings.TrimSpace(app.Spec.GetSource().TargetRevision),
		DestinationServer:    strings.TrimSpace(app.Spec.Destination.Server),
		DestinationNamespace: strings.TrimSpace(app.Spec.Destination.Namespace),
	}
	if app.Status.ReconciledAt != nil {
		inspection.ReconciledAt = app.Status.ReconciledAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if app.Status.OperationState != nil {
		inspection.OperationPhase = strings.TrimSpace(string(app.Status.OperationState.Phase))
		inspection.OperationMessage = strings.TrimSpace(app.Status.OperationState.Message)
	}
	inspection.IgnoreDifferenceTargets = buildIgnoreDifferenceTargets(app.Spec.IgnoreDifferences)
	inspection.RestartedAtIgnoreConfigured = inspectionHasRestartedAtIgnore(inspection.IgnoreDifferenceTargets)
	inspection.OutOfSyncResources, inspection.RestartedAtCandidates = summarizeApplicationResources(app.Status.Resources)
	return inspection
}

func buildIgnoreDifferenceTargets(items appv1.IgnoreDifferences) []IgnoreDifferenceTarget {
	if len(items) == 0 {
		return nil
	}
	targets := make([]IgnoreDifferenceTarget, 0, len(items))
	for _, item := range items {
		targets = append(targets, IgnoreDifferenceTarget{
			Group:        strings.TrimSpace(item.Group),
			Kind:         strings.TrimSpace(item.Kind),
			Name:         strings.TrimSpace(item.Name),
			Namespace:    strings.TrimSpace(item.Namespace),
			JSONPointers: cloneTrimmedStrings(item.JSONPointers),
		})
	}
	return targets
}

func summarizeApplicationResources(resources []appv1.ResourceStatus) ([]ApplicationResourceObservation, []ApplicationResourceObservation) {
	if len(resources) == 0 {
		return nil, nil
	}
	outOfSync := make([]ApplicationResourceObservation, 0)
	restartedAtCandidates := make([]ApplicationResourceObservation, 0)
	for _, resource := range resources {
		observation := ApplicationResourceObservation{
			Group:           strings.TrimSpace(resource.Group),
			Kind:            strings.TrimSpace(resource.Kind),
			Namespace:       strings.TrimSpace(resource.Namespace),
			Name:            strings.TrimSpace(resource.Name),
			SyncStatus:      strings.TrimSpace(string(resource.Status)),
			RequiresPruning: resource.RequiresPruning,
			SyncWave:        resource.SyncWave,
		}
		if resource.Health != nil {
			observation.HealthStatus = strings.TrimSpace(string(resource.Health.Status))
		}
		if strings.EqualFold(observation.SyncStatus, "OutOfSync") {
			outOfSync = append(outOfSync, observation)
		}
		if strings.EqualFold(observation.Kind, "Deployment") {
			restartedAtCandidates = append(restartedAtCandidates, observation)
		}
	}
	return outOfSync, restartedAtCandidates
}

func inspectionHasRestartedAtIgnore(targets []IgnoreDifferenceTarget) bool {
	for _, target := range targets {
		if !strings.EqualFold(target.Kind, "Deployment") {
			continue
		}
		for _, pointer := range target.JSONPointers {
			if strings.TrimSpace(pointer) == restartedAtIgnorePointer {
				return true
			}
		}
	}
	return false
}

func cloneTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func applyApplication(ctx context.Context, applications applicationAPI, app *appv1.Application) error {
	current, err := applications.Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = applications.Create(ctx, app, metav1.CreateOptions{})
			return err
		}
		return err
	}

	current.Spec = app.Spec
	current.Annotations = app.Annotations
	current.Labels = app.Labels

	_, err = applications.Update(ctx, current, metav1.UpdateOptions{})
	return err
}

func GetAppProject(ctx context.Context, name string) (*appv1.AppProject, error) {
	return Client.ArgoprojV1alpha1().AppProjects(namespace).Get(ctx, name, metav1.GetOptions{})
}

func UpdateAppProject(ctx context.Context, project *appv1.AppProject) error {
	_, err := Client.ArgoprojV1alpha1().AppProjects(namespace).Update(ctx, project, metav1.UpdateOptions{})
	return err
}
