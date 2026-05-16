package watch

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicinformer "k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var rolloutGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "rollouts",
}

type WorkloadCache interface {
	Start(ctx context.Context) error
	Ready() bool

	ListDeployments(namespace string, selector labels.Selector) ([]appsv1.Deployment, error)
	ListRollouts(namespace string, selector labels.Selector) ([]unstructured.Unstructured, error)
	ListPods(namespace string, selector labels.Selector) ([]corev1.Pod, error)
}

type WorkloadCacheConfig struct {
	Namespace    string
	ResyncPeriod time.Duration
}

type informerWorkloadCache struct {
	cfg WorkloadCacheConfig

	factory        informers.SharedInformerFactory
	dynamicFactory dynamicinformer.DynamicSharedInformerFactory

	deployInformer  cache.SharedIndexInformer
	podInformer     cache.SharedIndexInformer
	rolloutInformer cache.SharedIndexInformer

	deployLister appslisters.DeploymentLister
	podLister    corelisters.PodLister

	ready atomic.Bool
}

func NewWorkloadCache(restCfg *rest.Config, cfg WorkloadCacheConfig) (WorkloadCache, error) {
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	if cfg.ResyncPeriod <= 0 {
		cfg.ResyncPeriod = 10 * time.Minute
	}

	namespace := strings.TrimSpace(cfg.Namespace)
	var factory informers.SharedInformerFactory
	var dynamicFactory dynamicinformer.DynamicSharedInformerFactory
	if namespace == "" {
		factory = informers.NewSharedInformerFactory(clientset, cfg.ResyncPeriod)
		dynamicFactory = dynamicinformer.NewDynamicSharedInformerFactory(dyn, cfg.ResyncPeriod)
	} else {
		factory = informers.NewSharedInformerFactoryWithOptions(
			clientset,
			cfg.ResyncPeriod,
			informers.WithNamespace(namespace),
		)
		dynamicFactory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(
			dyn,
			cfg.ResyncPeriod,
			namespace,
			nil,
		)
	}

	return &informerWorkloadCache{
		cfg:             cfg,
		factory:         factory,
		dynamicFactory:  dynamicFactory,
		deployInformer:  factory.Apps().V1().Deployments().Informer(),
		podInformer:     factory.Core().V1().Pods().Informer(),
		rolloutInformer: dynamicFactory.ForResource(rolloutGVR).Informer(),
		deployLister:    factory.Apps().V1().Deployments().Lister(),
		podLister:       factory.Core().V1().Pods().Lister(),
	}, nil
}

func (c *informerWorkloadCache) Start(ctx context.Context) error {
	stopCh := ctx.Done()
	c.factory.Start(stopCh)
	c.dynamicFactory.Start(stopCh)

	if !cache.WaitForCacheSync(
		stopCh,
		c.deployInformer.HasSynced,
		c.podInformer.HasSynced,
		c.rolloutInformer.HasSynced,
	) {
		return fmt.Errorf("workload cache sync failed")
	}
	c.ready.Store(true)
	return nil
}

func (c *informerWorkloadCache) Ready() bool {
	return c.ready.Load()
}

func (c *informerWorkloadCache) ListDeployments(namespace string, selector labels.Selector) ([]appsv1.Deployment, error) {
	if !c.Ready() {
		return nil, fmt.Errorf("workload cache not ready")
	}
	var items []*appsv1.Deployment
	var err error
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		items, err = c.deployLister.List(selector)
	} else {
		items, err = c.deployLister.Deployments(namespace).List(selector)
	}
	if err != nil {
		return nil, err
	}
	out := make([]appsv1.Deployment, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, *item.DeepCopy())
	}
	return out, nil
}

func (c *informerWorkloadCache) ListPods(namespace string, selector labels.Selector) ([]corev1.Pod, error) {
	if !c.Ready() {
		return nil, fmt.Errorf("workload cache not ready")
	}
	var items []*corev1.Pod
	var err error
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		items, err = c.podLister.List(selector)
	} else {
		items, err = c.podLister.Pods(namespace).List(selector)
	}
	if err != nil {
		return nil, err
	}
	out := make([]corev1.Pod, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, *item.DeepCopy())
	}
	return out, nil
}

func (c *informerWorkloadCache) ListRollouts(namespace string, selector labels.Selector) ([]unstructured.Unstructured, error) {
	if !c.Ready() {
		return nil, fmt.Errorf("workload cache not ready")
	}
	namespace = strings.TrimSpace(namespace)
	items := c.rolloutInformer.GetStore().List()
	out := make([]unstructured.Unstructured, 0, len(items))
	for _, item := range items {
		obj, ok := item.(*unstructured.Unstructured)
		if !ok || obj == nil {
			continue
		}
		if namespace != "" && obj.GetNamespace() != namespace {
			continue
		}
		if selector != nil && !selector.Matches(labels.Set(obj.GetLabels())) {
			continue
		}
		out = append(out, *obj.DeepCopy())
	}
	return out, nil
}
