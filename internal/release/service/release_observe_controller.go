package service

import (
	"context"
	"strings"

	"github.com/bsonger/devflow-service/internal/platform/observer"
	platformobs "github.com/bsonger/devflow-service/internal/platform/runtime/observability"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type releaseObserveController struct {
	service *releaseService
}

func newReleaseObserveController(service *releaseService) *releaseObserveController {
	return &releaseObserveController{service: service}
}

func (c *releaseObserveController) runTerminal(ctx context.Context, release *model.Release, status model.ReleaseStatus) {
	if release == nil {
		return
	}
	log := platformobs.OperationLogger(ctx, "release_service", "mark_release_observation_terminal", "release",
		zap.String("resource_id", release.ID.String()),
		zap.String("status", string(status)),
	)
	switch status {
	case model.ReleaseSucceeded, model.ReleaseFailed, model.ReleaseRolledBack, model.ReleaseSyncFailed:
	default:
		return
	}
	appName := strings.TrimSpace(release.ArgoCDApplicationName)
	if appName == "" {
		log.Warn("skip release observation terminal update because argocd application name is empty")
		return
	}
	application, err := releaseGetArgoApplication(ctx, appName)
	if err != nil {
		log.Warn("load argocd application for terminal observe-state update failed",
			zap.String("argocd_application_name", appName),
			zap.Error(err),
		)
		return
	}
	if application == nil {
		log.Warn("skip release observation terminal update because argocd application is nil",
			zap.String("argocd_application_name", appName),
		)
		return
	}
	if application.Labels == nil {
		application.Labels = map[string]string{}
	}
	application.Labels[observer.ObserveStateLabel] = observer.ObserveStateDone
	if err := releaseUpdateArgoApplication(ctx, application); err != nil {
		log.Warn("update argocd application observe-state failed",
			zap.String("argocd_application_name", appName),
			zap.Error(err),
		)
		return
	}
	log.Info("updated argocd application observe-state",
		zap.String("argocd_application_name", appName),
		zap.String("observe_state", observer.ObserveStateDone),
	)
	c.markWorkloadsTerminal(ctx, release)
}

func (c *releaseObserveController) markWorkloadsTerminal(ctx context.Context, release *model.Release) {
	if release == nil {
		return
	}
	log := platformobs.OperationLogger(ctx, "release_service", "mark_release_workloads_observation_terminal", "release",
		zap.String("resource_id", release.ID.String()),
	)
	bundle, err := c.service.repoBundleStore().GetByReleaseID(ctx, release.ID)
	if err != nil {
		log.Warn("load release bundle for terminal observe-state update failed", zap.Error(err))
		return
	}
	if bundle == nil {
		log.Warn("skip release workload terminal update because bundle is nil")
		return
	}
	var (
		kubeClient    kubernetes.Interface
		dynamicClient dynamic.Interface
	)
	for _, item := range bundle.RenderedObjects {
		kind := strings.TrimSpace(item.Kind)
		name := strings.TrimSpace(item.Name)
		namespace := strings.TrimSpace(item.Namespace)
		if kind == "" || name == "" || namespace == "" {
			continue
		}
		switch kind {
		case "Deployment":
			if kubeClient == nil {
				client, err := releaseNewKubeClient()
				if err != nil {
					log.Warn("init kubernetes client for deployment observe-state update failed", zap.Error(err))
					return
				}
				kubeClient = client
			}
			workload, err := kubeClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				log.Warn("load deployment for terminal observe-state update failed",
					zap.String("workload_kind", kind),
					zap.String("workload_name", name),
					zap.String("namespace", namespace),
					zap.Error(err),
				)
				continue
			}
			if workload.Labels == nil {
				workload.Labels = map[string]string{}
			}
			workload.Labels[observer.ObserveStateLabel] = observer.ObserveStateDone
			if workload.Spec.Template.Labels == nil {
				workload.Spec.Template.Labels = map[string]string{}
			}
			workload.Spec.Template.Labels[observer.ObserveStateLabel] = observer.ObserveStateDone
			if _, err := kubeClient.AppsV1().Deployments(namespace).Update(ctx, workload, metav1.UpdateOptions{}); err != nil {
				log.Warn("update deployment observe-state failed",
					zap.String("workload_kind", kind),
					zap.String("workload_name", name),
					zap.String("namespace", namespace),
					zap.Error(err),
				)
				continue
			}
			log.Info("updated workload observe-state",
				zap.String("workload_kind", kind),
				zap.String("workload_name", name),
				zap.String("namespace", namespace),
				zap.String("observe_state", observer.ObserveStateDone),
			)
		case "Rollout":
			if dynamicClient == nil {
				client, err := releaseNewDynamicClient()
				if err != nil {
					log.Warn("init dynamic client for rollout observe-state update failed", zap.Error(err))
					return
				}
				dynamicClient = client
			}
			resource := dynamicClient.Resource(releaseRolloutGVR).Namespace(namespace)
			workload, err := resource.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				log.Warn("load rollout for terminal observe-state update failed",
					zap.String("workload_kind", kind),
					zap.String("workload_name", name),
					zap.String("namespace", namespace),
					zap.Error(err),
				)
				continue
			}
			labels := workload.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			labels[observer.ObserveStateLabel] = observer.ObserveStateDone
			workload.SetLabels(labels)

			templateLabels, _, _ := unstructured.NestedStringMap(workload.Object, "spec", "template", "metadata", "labels")
			if templateLabels == nil {
				templateLabels = map[string]string{}
			}
			templateLabels[observer.ObserveStateLabel] = observer.ObserveStateDone
			if err := unstructured.SetNestedStringMap(workload.Object, templateLabels, "spec", "template", "metadata", "labels"); err != nil {
				log.Warn("update rollout pod template observe-state failed",
					zap.String("workload_kind", kind),
					zap.String("workload_name", name),
					zap.String("namespace", namespace),
					zap.Error(err),
				)
				continue
			}
			if _, err := resource.Update(ctx, workload, metav1.UpdateOptions{}); err != nil {
				log.Warn("update rollout observe-state failed",
					zap.String("workload_kind", kind),
					zap.String("workload_name", name),
					zap.String("namespace", namespace),
					zap.Error(err),
				)
				continue
			}
			log.Info("updated workload observe-state",
				zap.String("workload_kind", kind),
				zap.String("workload_name", name),
				zap.String("namespace", namespace),
				zap.String("observe_state", observer.ObserveStateDone),
			)
		}
	}
}

var releaseRolloutGVR = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "rollouts"}
