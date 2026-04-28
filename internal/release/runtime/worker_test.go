package runtime

import (
	"context"
	"os"
	"testing"
	"time"

	model "github.com/bsonger/devflow-service/internal/release/domain"
)

type stubReleaseIntentProcessor struct {
	processFn func(context.Context, string, time.Duration) (bool, error)
}

func (s stubReleaseIntentProcessor) ProcessNextReleaseIntent(ctx context.Context, workerID string, leaseDuration time.Duration) (bool, error) {
	return s.processFn(ctx, workerID, leaseDuration)
}

func TestReleaseIntentWorkerConfigFromModelDefaults(t *testing.T) {
	cfg := ReleaseIntentWorkerConfigFromModel(nil)
	if cfg.Enabled {
		t.Fatal("expected worker disabled by default")
	}
	if cfg.LeaseDuration != time.Minute {
		t.Fatalf("lease_duration = %s want %s", cfg.LeaseDuration, time.Minute)
	}
	if cfg.PollInterval != 5*time.Second {
		t.Fatalf("poll_interval = %s want %s", cfg.PollInterval, 5*time.Second)
	}
	host, _ := os.Hostname()
	if host != "" && cfg.WorkerID != "release-worker@"+host {
		t.Fatalf("worker_id = %q", cfg.WorkerID)
	}
}

func TestReleaseIntentWorkerConfigFromModelOverridesValues(t *testing.T) {
	cfg := ReleaseIntentWorkerConfigFromModel(&model.WorkerConfig{
		Enabled:              true,
		WorkerID:             "worker-a",
		LeaseDurationSeconds: 90,
		PollIntervalSeconds:  2,
	})
	if !cfg.Enabled {
		t.Fatal("expected worker enabled")
	}
	if cfg.WorkerID != "worker-a" {
		t.Fatalf("worker_id = %q", cfg.WorkerID)
	}
	if cfg.LeaseDuration != 90*time.Second {
		t.Fatalf("lease_duration = %s", cfg.LeaseDuration)
	}
	if cfg.PollInterval != 2*time.Second {
		t.Fatalf("poll_interval = %s", cfg.PollInterval)
	}
}

func TestStartReleaseIntentWorkerRunsProcessor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	called := make(chan struct{}, 1)
	StartReleaseIntentWorker(ctx, ReleaseIntentWorkerConfig{
		Enabled:       true,
		WorkerID:      "worker-a",
		LeaseDuration: time.Minute,
		PollInterval:  10 * time.Millisecond,
	}, stubReleaseIntentProcessor{
		processFn: func(_ context.Context, workerID string, leaseDuration time.Duration) (bool, error) {
			if workerID != "worker-a" {
				t.Fatalf("workerID = %q", workerID)
			}
			if leaseDuration != time.Minute {
				t.Fatalf("leaseDuration = %s", leaseDuration)
			}
			select {
			case called <- struct{}{}:
			default:
			}
			cancel()
			return false, nil
		},
	})

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("worker did not invoke processor")
	}
}
