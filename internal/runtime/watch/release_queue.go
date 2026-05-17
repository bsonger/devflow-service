package watch

import (
	"context"
	"errors"
	"time"

	"k8s.io/client-go/util/workqueue"
)

type ReleaseQueue interface {
	Add(releaseID string)
	Run(ctx context.Context, workers int, handler func(context.Context, string) error)
	ShutDown()
}

type releaseQueue struct {
	queue workqueue.TypedRateLimitingInterface[string]
}

type requeueAfterError interface {
	error
	RequeueAfter() time.Duration
}

func NewReleaseQueue() ReleaseQueue {
	return &releaseQueue{
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name: "runtime-release-reconcile",
			},
		),
	}
}

func (r *releaseQueue) Add(releaseID string) {
	if releaseID == "" {
		return
	}
	r.queue.Add(releaseID)
}

func (r *releaseQueue) ShutDown() {
	r.queue.ShutDown()
}

func (r *releaseQueue) Run(ctx context.Context, workers int, handler func(context.Context, string) error) {
	if workers <= 0 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		go func() {
			for {
				item, shutdown := r.queue.Get()
				if shutdown {
					return
				}
				func() {
					defer r.queue.Done(item)
					runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()
					if err := handler(runCtx, item); err != nil {
						var requeueErr requeueAfterError
						if errors.As(err, &requeueErr) {
							r.queue.Forget(item)
							r.queue.AddAfter(item, requeueErr.RequeueAfter())
							return
						}
						r.queue.AddRateLimited(item)
						return
					}
					r.queue.Forget(item)
				}()
			}
		}()
	}
	<-ctx.Done()
	r.queue.ShutDown()
}
