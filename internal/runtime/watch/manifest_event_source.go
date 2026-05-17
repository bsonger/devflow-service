package watch

import (
	"fmt"
	"strings"

	"github.com/bsonger/devflow-service/internal/platform/observer"
	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/client-go/tools/cache"
)

type ManifestEventSource struct {
	cache          InformerTektonCache
	queue          ManifestQueue
	controlPlaneID string
}

func NewManifestEventSource(cache InformerTektonCache, queue ManifestQueue, controlPlaneID string) *ManifestEventSource {
	return &ManifestEventSource{
		cache:          cache,
		queue:          queue,
		controlPlaneID: strings.TrimSpace(controlPlaneID),
	}
}

func (s *ManifestEventSource) Start() error {
	if s == nil || s.cache == nil || s.queue == nil {
		return fmt.Errorf("manifest event source is not configured")
	}
	return s.cache.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			_ = s.handleObject(obj)
		},
		UpdateFunc: func(_, newObj any) {
			_ = s.handleObject(newObj)
		},
		DeleteFunc: func(obj any) {
			_ = s.handleObject(obj)
		},
	})
}

func (s *ManifestEventSource) handleObject(obj any) error {
	if s == nil || s.queue == nil {
		return nil
	}

	switch item := obj.(type) {
	case *tknv1.PipelineRun:
		if cacheImpl, ok := s.cache.(*informerTektonCache); ok {
			cacheImpl.upsertPipelineRun(item)
		}
		manifestID := strings.TrimSpace(item.Labels[manifestIDLabel])
		if manifestID == "" {
			return nil
		}
		if s.controlPlaneID != "" && strings.TrimSpace(item.Labels[releasedomain.ControlPlaneLabel]) != s.controlPlaneID {
			return nil
		}
		if strings.TrimSpace(item.Labels[observer.ObserveStateLabel]) == observer.ObserveStateDone {
			return nil
		}
		s.queue.Add(manifestID)
		return nil
	case *tknv1.TaskRun:
		if cacheImpl, ok := s.cache.(*informerTektonCache); ok {
			cacheImpl.upsertTaskRun(item)
		}
		pipelineRunName := taskRunPipelineName(item)
		if pipelineRunName == "" || s.cache == nil {
			return nil
		}
		for _, manifestID := range s.cache.ListManifestIDs(s.controlPlaneID) {
			snapshot, ok := s.cache.GetManifestSnapshot(manifestID)
			if !ok || snapshot == nil {
				continue
			}
			for _, pipeline := range snapshot.PipelineRuns {
				if strings.TrimSpace(pipeline.Name) == pipelineRunName && strings.TrimSpace(pipeline.Namespace) == strings.TrimSpace(item.Namespace) {
					s.queue.Add(manifestID)
					return nil
				}
			}
		}
	}
	return nil
}
