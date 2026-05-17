package watch

import (
	"context"
	"fmt"
	"strings"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

type ReleaseEventSource struct {
	cache          WorkloadCache
	queue          ReleaseQueue
	controlPlaneID string
}

func NewReleaseEventSource(cache WorkloadCache, queue ReleaseQueue, controlPlaneID string) *ReleaseEventSource {
	return &ReleaseEventSource{
		cache:          cache,
		queue:          queue,
		controlPlaneID: strings.TrimSpace(controlPlaneID),
	}
}

func (s *ReleaseEventSource) Run(ctx context.Context) {
	if s == nil || s.cache == nil || s.queue == nil {
		<-ctx.Done()
		return
	}
	if err := s.cache.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { s.handleObject(obj) },
		UpdateFunc: func(_, newObj any) { s.handleObject(newObj) },
		DeleteFunc: func(obj any) { s.handleObject(obj) },
	}); err != nil {
		<-ctx.Done()
		return
	}
	if err := s.cache.Start(ctx); err != nil {
		return
	}
	<-ctx.Done()
}

func (s *ReleaseEventSource) handleObject(obj any) {
	if s == nil || s.queue == nil {
		return
	}
	labels := releaseLabelsFromObject(obj)
	releaseID := strings.TrimSpace(labels[releasedomain.ReleaseIDLabel])
	if releaseID == "" {
		return
	}
	if s.controlPlaneID != "" && strings.TrimSpace(labels[releasedomain.ControlPlaneLabel]) != s.controlPlaneID {
		return
	}
	if strings.TrimSpace(labels[releasedomain.ReleaseStatusLabel]) != string(releasedomain.ReleaseRunning) {
		return
	}
	s.queue.Add(releaseID)
}

func releaseLabelsFromObject(obj any) map[string]string {
	switch item := obj.(type) {
	case *appsv1.Deployment:
		if item == nil {
			return nil
		}
		return item.GetLabels()
	case *corev1.Pod:
		if item == nil {
			return nil
		}
		return item.GetLabels()
	case *unstructured.Unstructured:
		if item == nil {
			return nil
		}
		return item.GetLabels()
	case cache.DeletedFinalStateUnknown:
		return releaseLabelsFromObject(item.Obj)
	default:
		return nil
	}
}

func EnsureReleaseEventSourceConfigured(source *ReleaseEventSource) error {
	if source == nil || source.cache == nil || source.queue == nil {
		return fmt.Errorf("release event source is not configured")
	}
	return nil
}
