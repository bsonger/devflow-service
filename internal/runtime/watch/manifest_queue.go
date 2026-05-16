package watch

import (
	"context"
	"time"

	"k8s.io/client-go/util/workqueue"
)

type ManifestQueue interface {
	Add(manifestID string)
	Run(ctx context.Context, workers int, handler func(context.Context, string) error)
	ShutDown()
}

type manifestQueue struct {
	queue workqueue.TypedRateLimitingInterface[string]
}

func NewManifestQueue() ManifestQueue {
	return &manifestQueue{
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name: "runtime-manifest-reconcile",
			},
		),
	}
}

func (q *manifestQueue) Add(manifestID string) {
	if manifestID == "" {
		return
	}
	q.queue.Add(manifestID)
}

func (q *manifestQueue) ShutDown() {
	q.queue.ShutDown()
}

func (q *manifestQueue) Run(ctx context.Context, workers int, handler func(context.Context, string) error) {
	if workers <= 0 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		go func() {
			for {
				item, shutdown := q.queue.Get()
				if shutdown {
					return
				}
				func() {
					defer q.queue.Done(item)
					runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()
					if err := handler(runCtx, item); err != nil {
						q.queue.AddRateLimited(item)
						return
					}
					q.queue.Forget(item)
				}()
			}
		}()
	}
	<-ctx.Done()
	q.queue.ShutDown()
}
