package watch

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	releasedomain "github.com/bsonger/devflow-service/internal/release/domain"
	tknv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektoninformers "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type InformerTektonCacheConfig struct {
	Namespace    string
	PipelineName string
	ResyncPeriod time.Duration
}

type InformerTektonCache interface {
	TektonCache
	Start(context.Context) error
	Ready() bool
	AddEventHandler(cache.ResourceEventHandler) error
}

type informerTektonCache struct {
	mu           sync.RWMutex
	pipelineName string

	factory          tektoninformers.SharedInformerFactory
	pipelineInformer cache.SharedIndexInformer
	taskInformer     cache.SharedIndexInformer

	pipelineRuns  map[string]*tknv1.PipelineRun
	taskRuns      map[string]*tknv1.TaskRun
	manifestIndex map[string]map[string]struct{}
	taskRunIndex  map[string]map[string]struct{}

	ready atomic.Bool
}

func NewInformerTektonCache(restCfg *rest.Config, cfg InformerTektonCacheConfig) (InformerTektonCache, error) {
	clientset, err := tektonclient.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	if cfg.ResyncPeriod <= 0 {
		cfg.ResyncPeriod = 10 * time.Minute
	}

	namespace := strings.TrimSpace(cfg.Namespace)
	var factory tektoninformers.SharedInformerFactory
	if namespace == "" {
		factory = tektoninformers.NewSharedInformerFactory(clientset, cfg.ResyncPeriod)
	} else {
		factory = tektoninformers.NewSharedInformerFactoryWithOptions(
			clientset,
			cfg.ResyncPeriod,
			tektoninformers.WithNamespace(namespace),
		)
	}

	return &informerTektonCache{
		pipelineName:     strings.TrimSpace(cfg.PipelineName),
		factory:          factory,
		pipelineInformer: factory.Tekton().V1().PipelineRuns().Informer(),
		taskInformer:     factory.Tekton().V1().TaskRuns().Informer(),
		pipelineRuns:     map[string]*tknv1.PipelineRun{},
		taskRuns:         map[string]*tknv1.TaskRun{},
		manifestIndex:    map[string]map[string]struct{}{},
		taskRunIndex:     map[string]map[string]struct{}{},
	}, nil
}

func newInformerTektonCacheForTest(pipelineName string) *informerTektonCache {
	return &informerTektonCache{
		pipelineName:  strings.TrimSpace(pipelineName),
		pipelineRuns:  map[string]*tknv1.PipelineRun{},
		taskRuns:      map[string]*tknv1.TaskRun{},
		manifestIndex: map[string]map[string]struct{}{},
		taskRunIndex:  map[string]map[string]struct{}{},
	}
}

func (c *informerTektonCache) Start(ctx context.Context) error {
	if c == nil || c.factory == nil || c.pipelineInformer == nil || c.taskInformer == nil {
		return fmt.Errorf("tekton informer cache is not configured")
	}

	if _, err := c.pipelineInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) { c.handlePipelineObject(obj, false) },
		UpdateFunc: func(_, newObj any) {
			c.handlePipelineObject(newObj, false)
		},
		DeleteFunc: func(obj any) { c.handlePipelineObject(obj, true) },
	}); err != nil {
		return err
	}
	if _, err := c.taskInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) { c.handleTaskObject(obj, false) },
		UpdateFunc: func(_, newObj any) {
			c.handleTaskObject(newObj, false)
		},
		DeleteFunc: func(obj any) { c.handleTaskObject(obj, true) },
	}); err != nil {
		return err
	}

	stopCh := ctx.Done()
	c.factory.Start(stopCh)
	if !cache.WaitForCacheSync(stopCh, c.pipelineInformer.HasSynced, c.taskInformer.HasSynced) {
		return fmt.Errorf("tekton informer cache sync failed")
	}
	c.ready.Store(true)
	return nil
}

func (c *informerTektonCache) Ready() bool {
	return c != nil && c.ready.Load()
}

func (c *informerTektonCache) AddEventHandler(handler cache.ResourceEventHandler) error {
	if c == nil || handler == nil {
		return nil
	}
	if c.pipelineInformer == nil || c.taskInformer == nil {
		return fmt.Errorf("tekton informer cache is not configured")
	}
	if _, err := c.pipelineInformer.AddEventHandler(handler); err != nil {
		return err
	}
	if _, err := c.taskInformer.AddEventHandler(handler); err != nil {
		return err
	}
	return nil
}

func (c *informerTektonCache) ListManifestIDs(controlPlaneID string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	controlPlaneID = strings.TrimSpace(controlPlaneID)
	seen := map[string]struct{}{}
	out := make([]string, 0, len(c.manifestIndex))
	for manifestID, keys := range c.manifestIndex {
		if manifestID == "" || len(keys) == 0 {
			continue
		}
		for key := range keys {
			pr := c.pipelineRuns[key]
			if pr == nil {
				continue
			}
			if controlPlaneID != "" && strings.TrimSpace(pr.Labels[releasedomain.ControlPlaneLabel]) != controlPlaneID {
				continue
			}
			if _, ok := seen[manifestID]; ok {
				break
			}
			seen[manifestID] = struct{}{}
			out = append(out, manifestID)
			break
		}
	}
	sort.Strings(out)
	return out
}

func (c *informerTektonCache) GetManifestSnapshot(manifestID string) (*TektonSnapshot, bool) {
	manifestID = strings.TrimSpace(manifestID)
	if manifestID == "" {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := c.manifestIndex[manifestID]
	if len(keys) == 0 {
		return nil, false
	}

	pipelineRuns := make([]PipelineRunSnapshot, 0, len(keys))
	for key := range keys {
		pr := c.pipelineRuns[key]
		snapshot, ok := snapshotPipelineRun(pr)
		if !ok {
			continue
		}
		pipelineRuns = append(pipelineRuns, snapshot)
	}
	if len(pipelineRuns) == 0 {
		return nil, false
	}
	sort.SliceStable(pipelineRuns, func(i, j int) bool {
		return moreRecentPipelineRun(pipelineRuns[i], pipelineRuns[j])
	})

	taskKeys := c.taskRunIndex[manifestID]
	taskRuns := make([]*tknv1.TaskRun, 0, len(taskKeys))
	for key := range taskKeys {
		if tr := c.taskRuns[key]; tr != nil {
			taskRuns = append(taskRuns, tr)
		}
	}

	manifestTasks := make([]TaskRunSnapshot, 0)
	for _, pipeline := range pipelineRuns {
		pr := c.pipelineRuns[pipelineRunObjectKey(pipeline.Namespace, pipeline.Name)]
		manifestTasks = append(manifestTasks, taskRunSnapshotsForPipeline(manifestID, pr, taskRuns)...)
	}

	return &TektonSnapshot{
		PipelineRuns: append([]PipelineRunSnapshot(nil), pipelineRuns...),
		TaskRuns: map[string][]TaskRunSnapshot{
			manifestID: append([]TaskRunSnapshot(nil), manifestTasks...),
		},
	}, true
}

func (c *informerTektonCache) handlePipelineObject(obj any, deleted bool) {
	pr, ok := pipelineRunFromObject(obj)
	if !ok || pr == nil {
		return
	}
	if deleted {
		c.deletePipelineRun(pr)
		return
	}
	c.upsertPipelineRun(pr)
}

func (c *informerTektonCache) handleTaskObject(obj any, deleted bool) {
	tr, ok := taskRunFromObject(obj)
	if !ok || tr == nil {
		return
	}
	if deleted {
		c.deleteTaskRun(tr)
		return
	}
	c.upsertTaskRun(tr)
}

func (c *informerTektonCache) upsertPipelineRun(pr *tknv1.PipelineRun) {
	if c == nil || pr == nil || !c.pipelineMatches(pr) {
		return
	}
	key := pipelineRunKey(pr)
	manifestID := strings.TrimSpace(pr.Labels[manifestIDLabel])
	if key == "" || manifestID == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing := c.pipelineRuns[key]; existing != nil {
		c.removePipelineIndexLocked(key, strings.TrimSpace(existing.Labels[manifestIDLabel]))
	}
	c.pipelineRuns[key] = clonePipelineRun(pr)
	if c.manifestIndex[manifestID] == nil {
		c.manifestIndex[manifestID] = map[string]struct{}{}
	}
	c.manifestIndex[manifestID][key] = struct{}{}
}

func (c *informerTektonCache) deletePipelineRun(pr *tknv1.PipelineRun) {
	if c == nil || pr == nil {
		return
	}
	key := pipelineRunKey(pr)
	if key == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	existing := c.pipelineRuns[key]
	if existing == nil {
		return
	}
	delete(c.pipelineRuns, key)
	c.removePipelineIndexLocked(key, strings.TrimSpace(existing.Labels[manifestIDLabel]))
}

func (c *informerTektonCache) upsertTaskRun(tr *tknv1.TaskRun) {
	if c == nil || tr == nil {
		return
	}
	key := pipelineRunObjectKey(tr.Namespace, tr.Name)
	pipelineKey := taskRunPipelineKey(tr)
	if key == "" || pipelineKey == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	manifestID := ""
	if pr := c.pipelineRuns[pipelineKey]; pr != nil {
		manifestID = strings.TrimSpace(pr.Labels[manifestIDLabel])
	}
	if manifestID == "" {
		return
	}

	if existing := c.taskRuns[key]; existing != nil {
		c.removeTaskIndexLocked(key, c.manifestIDForTaskRunLocked(existing))
	}
	c.taskRuns[key] = cloneTaskRun(tr)
	if c.taskRunIndex[manifestID] == nil {
		c.taskRunIndex[manifestID] = map[string]struct{}{}
	}
	c.taskRunIndex[manifestID][key] = struct{}{}
}

func (c *informerTektonCache) deleteTaskRun(tr *tknv1.TaskRun) {
	if c == nil || tr == nil {
		return
	}
	key := pipelineRunObjectKey(tr.Namespace, tr.Name)
	if key == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	existing := c.taskRuns[key]
	if existing == nil {
		return
	}
	delete(c.taskRuns, key)
	c.removeTaskIndexLocked(key, c.manifestIDForTaskRunLocked(existing))
}

func (c *informerTektonCache) manifestIDForTaskRunLocked(tr *tknv1.TaskRun) string {
	if tr == nil {
		return ""
	}
	pr := c.pipelineRuns[taskRunPipelineKey(tr)]
	if pr == nil {
		return ""
	}
	return strings.TrimSpace(pr.Labels[manifestIDLabel])
}

func (c *informerTektonCache) removePipelineIndexLocked(key, manifestID string) {
	if manifestID == "" {
		return
	}
	delete(c.manifestIndex[manifestID], key)
	if len(c.manifestIndex[manifestID]) == 0 {
		delete(c.manifestIndex, manifestID)
	}
}

func (c *informerTektonCache) removeTaskIndexLocked(key, manifestID string) {
	if manifestID == "" {
		return
	}
	delete(c.taskRunIndex[manifestID], key)
	if len(c.taskRunIndex[manifestID]) == 0 {
		delete(c.taskRunIndex, manifestID)
	}
}

func (c *informerTektonCache) pipelineMatches(pr *tknv1.PipelineRun) bool {
	filter := strings.TrimSpace(c.pipelineName)
	if filter == "" {
		return true
	}
	if pr == nil || pr.Spec.PipelineRef == nil {
		return false
	}
	return strings.TrimSpace(pr.Spec.PipelineRef.Name) == filter
}

func pipelineRunFromObject(obj any) (*tknv1.PipelineRun, bool) {
	switch item := obj.(type) {
	case *tknv1.PipelineRun:
		return item, true
	case cache.DeletedFinalStateUnknown:
		pr, ok := item.Obj.(*tknv1.PipelineRun)
		return pr, ok
	default:
		return nil, false
	}
}

func taskRunFromObject(obj any) (*tknv1.TaskRun, bool) {
	switch item := obj.(type) {
	case *tknv1.TaskRun:
		return item, true
	case cache.DeletedFinalStateUnknown:
		tr, ok := item.Obj.(*tknv1.TaskRun)
		return tr, ok
	default:
		return nil, false
	}
}
