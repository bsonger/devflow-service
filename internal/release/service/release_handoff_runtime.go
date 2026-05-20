package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argoutil "github.com/argoproj/argo-cd/v3/util/argo"
	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	"github.com/bsonger/devflow-service/internal/platform/observer"
	"github.com/bsonger/devflow-service/internal/platform/oci"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	argoclient "github.com/bsonger/devflow-service/internal/release/transport/argo"
	"github.com/bsonger/devflow-service/internal/shared/errs"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type releaseHandoffController struct {
	service *releaseService
}

func newReleaseHandoffController(service *releaseService) *releaseHandoffController {
	return &releaseHandoffController{service: service}
}

func (c *releaseHandoffController) run(ctx context.Context, release *model.Release, manifest *manifestdomain.Manifest, app *releasesupport.ApplicationProjection, target releasesupport.DeployTarget) error {
	start := time.Now()
	log := platformobs.OperationLogger(ctx, "release_service", "create_argocd_application", "release",
		zap.String("resource_id", release.ID.String()),
	)
	application := buildArgoApplication(release, manifest, app, target)
	if err := c.service.UpdateStep(ctx, release.ID, "create_argocd_application", model.StepRunning, 25, createArgoApplicationStartMessage(release, application.Name, target), nil, nil); err != nil {
		return err
	}
	if err := c.persistArgoApplicationMetadata(ctx, release, application.Name); err != nil {
		return err
	}
	applyReleaseApplicationMetadata(ctx, release, application)

	err := applyReleaseApplication(ctx, release.Type, application, argoclient.CreateApplication, argoclient.UpdateApplication, c.syncArgoApplication)
	if err != nil {
		_ = c.service.UpdateStep(ctx, release.ID, "create_argocd_application", model.StepFailed, 100, createArgoApplicationFailureMessage(application.Name, err), nil, nil)
		observeArgoApplicationCreate(ctx, release, false, time.Since(start))
		log.Error("argo sync failed", zap.String("result", "error"), zap.Error(err))
		return err
	}
	_ = c.service.UpdateStep(ctx, release.ID, "create_argocd_application", model.StepSucceeded, 100, createArgoApplicationSuccessMessage(release, application.Name), nil, nil)
	observeArgoApplicationCreate(ctx, release, true, time.Since(start))
	if code, message := releaseDeploymentStartStep(release); code != "" {
		_ = c.service.UpdateStep(ctx, release.ID, code, model.StepSucceeded, 100, message, nil, nil)
	}
	return nil
}

func applyReleaseApplicationMetadata(ctx context.Context, release *model.Release, application *appv1.Application) {
	if application == nil {
		return
	}
	sc := trace.SpanContextFromContext(ctx)
	application.Annotations = map[string]string{
		oci.TraceIDAnnotation:             sc.TraceID().String(),
		oci.SpanAnnotation:                sc.SpanID().String(),
		observer.ObserveKindAnnotation:    observer.ObserveKindRelease,
		observer.ObserveOwnerIDAnnotation: release.ID.String(),
	}
	application.Labels = map[string]string{
		model.ReleaseStatusLabel:      string(model.ReleaseRunning),
		"app.kubernetes.io/name":      application.Name,
		model.ReleaseIDLabel:          release.ID.String(),
		model.ReleaseApplicationLabel: release.ApplicationID.String(),
		model.ReleaseEnvironmentLabel: releaseTargetEnvironment(release),
		observer.ObserveStateLabel:    observer.ObserveStateRunning,
	}
	if controlPlaneID := strings.TrimSpace(releasesupport.CurrentRuntimeConfig().ControlPlaneID); controlPlaneID != "" {
		application.Labels[model.ControlPlaneLabel] = controlPlaneID
	}
}

func createArgoApplicationStartMessage(release *model.Release, appName string, target releasesupport.DeployTarget) string {
	appName = strings.TrimSpace(appName)
	environmentID := releaseTargetEnvironment(release)
	namespace := strings.TrimSpace(target.Namespace)
	switch {
	case appName != "" && environmentID != "" && namespace != "":
		return fmt.Sprintf("creating argocd application %s for environment %s in namespace %s", appName, environmentID, namespace)
	case appName != "" && environmentID != "":
		return fmt.Sprintf("creating argocd application %s for environment %s", appName, environmentID)
	case appName != "":
		return fmt.Sprintf("creating argocd application %s", appName)
	default:
		return "creating argocd application"
	}
}

func createArgoApplicationSuccessMessage(release *model.Release, appName string) string {
	appName = strings.TrimSpace(appName)
	environmentID := releaseTargetEnvironment(release)
	artifactRef := releaseExecutionArtifactRef(release)
	switch {
	case appName != "" && environmentID != "" && artifactRef != "":
		return fmt.Sprintf("argocd application %s created for environment %s and sync requested from %s", appName, environmentID, artifactRef)
	case appName != "" && artifactRef != "":
		return fmt.Sprintf("argocd application %s created and sync requested from %s", appName, artifactRef)
	case appName != "" && environmentID != "":
		return fmt.Sprintf("argocd application %s created for environment %s and sync requested", appName, environmentID)
	case appName != "":
		return fmt.Sprintf("argocd application %s created and sync requested", appName)
	default:
		return "argocd application created and sync requested"
	}
}

func createArgoApplicationFailureMessage(appName string, err error) string {
	appName = strings.TrimSpace(appName)
	if err == nil {
		if appName != "" {
			return fmt.Sprintf("argocd application %s failed", appName)
		}
		return "argocd application failed"
	}
	if appName != "" {
		return fmt.Sprintf("argocd application %s failed: %s", appName, err.Error())
	}
	return err.Error()
}

func (c *releaseHandoffController) persistArgoApplicationMetadata(ctx context.Context, release *model.Release, appName string) error {
	appName = strings.TrimSpace(appName)
	if release == nil || appName == "" {
		return nil
	}
	if release.ArgoCDApplicationName == appName && release.ExternalRef == appName {
		return nil
	}
	updatedAt := time.Now()
	release.ArgoCDApplicationName = appName
	release.ExternalRef = appName
	release.UpdatedAt = updatedAt
	return c.service.repoStore().UpdateArgoMetadata(ctx, release.ID, appName, appName, updatedAt)
}

func releaseDeploymentStartStep(release *model.Release) (string, string) {
	if release == nil {
		return "", ""
	}
	switch model.ReleaseStrategyToType(release.Strategy) {
	case model.BlueGreen:
		return "deploy_preview", "preview deployment started"
	case model.Canary:
		return "deploy_canary", "canary deployment started"
	default:
		return "start_deployment", "deployment sync started"
	}
}

func applyReleaseApplication(ctx context.Context, releaseType string, application *appv1.Application, createFn func(context.Context, *appv1.Application) error, updateFn func(context.Context, *appv1.Application) error, syncFn func(context.Context, string) error) error {
	switch releaseType {
	case model.ReleaseInstall:
		if err := createFn(ctx, application); err != nil {
			return err
		}
	case model.ReleaseUpgrade, model.ReleaseRollback:
		if err := updateFn(ctx, application); err != nil {
			return err
		}
	default:
		return errs.InvalidArgument("unknown release type")
	}
	return syncFn(ctx, application.Name)
}

func (c *releaseHandoffController) syncArgoApplication(ctx context.Context, appName string) error {
	applications := argoclient.Client.ArgoprojV1alpha1().Applications("argocd")
	_, err := argoutil.SetAppOperation(applications, appName, buildReleaseSyncOperation())
	return err
}
