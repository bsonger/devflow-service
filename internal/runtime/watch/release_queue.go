package watch

import (
	"context"
	"errors"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	"go.uber.org/zap"
	"k8s.io/client-go/util/workqueue"
)

const runtimeReleaseQueueName = "runtime-release-reconcile"

var releaseQueueLogf = defaultReleaseQueueLogf

func defaultReleaseQueueLogf(event string, fields ...zap.Field) {
	logger.RootLogger.Named("runtime.state").Info(event, fields...)
}

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
				Name: runtimeReleaseQueueName,
			},
		),
	}
}

func (r *releaseQueue) Add(releaseID string) {
	if releaseID == "" {
		return
	}
	releaseQueueLogf("queue_add",
		zap.String("queue", runtimeReleaseQueueName),
		zap.String("release_id", releaseID),
	)
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
					releaseQueueLogf("queue_handle_start",
						zap.String("queue", runtimeReleaseQueueName),
						zap.String("release_id", item),
						zap.Int("workers", workers),
					)
					if err := handler(runCtx, item); err != nil {
						var requeueErr requeueAfterError
						if errors.As(err, &requeueErr) {
							releaseQueueLogf("queue_handle_requeue_after",
								zap.String("queue", runtimeReleaseQueueName),
								zap.String("release_id", item),
								zap.Int("workers", workers),
								zap.Duration("requeue_after", requeueErr.RequeueAfter()),
							)
							r.queue.Forget(item)
							r.queue.AddAfter(item, requeueErr.RequeueAfter())
							return
						}
						releaseQueueLogf("queue_handle_rate_limited",
							zap.String("queue", runtimeReleaseQueueName),
							zap.String("release_id", item),
							zap.Int("workers", workers),
						)
						r.queue.AddRateLimited(item)
						return
					}
					releaseQueueLogf("queue_handle_done",
						zap.String("queue", runtimeReleaseQueueName),
						zap.String("release_id", item),
						zap.Int("workers", workers),
					)
					r.queue.Forget(item)
				}()
			}
		}()
	}
	<-ctx.Done()
	r.queue.ShutDown()
}
