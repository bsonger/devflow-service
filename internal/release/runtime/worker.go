package runtime

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/bsonger/devflow-service/internal/platform/logger"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"go.uber.org/zap"
)

type ReleaseIntentProcessor interface {
	ProcessNextReleaseIntent(ctx context.Context, workerID string, leaseDuration time.Duration) (bool, error)
}

type ReleaseIntentWorkerConfig struct {
	Enabled       bool
	WorkerID      string
	LeaseDuration time.Duration
	PollInterval  time.Duration
}

func ReleaseIntentWorkerConfigFromModel(cfg *model.WorkerConfig) ReleaseIntentWorkerConfig {
	workerID := "release-worker"
	if host, err := os.Hostname(); err == nil && strings.TrimSpace(host) != "" {
		workerID = "release-worker@" + strings.TrimSpace(host)
	}
	out := ReleaseIntentWorkerConfig{
		Enabled:       false,
		WorkerID:      workerID,
		LeaseDuration: time.Minute,
		PollInterval:  5 * time.Second,
	}
	if cfg == nil {
		return out
	}
	out.Enabled = cfg.Enabled
	if value := strings.TrimSpace(cfg.WorkerID); value != "" {
		out.WorkerID = value
	}
	if cfg.LeaseDurationSeconds > 0 {
		out.LeaseDuration = time.Duration(cfg.LeaseDurationSeconds) * time.Second
	}
	if cfg.PollIntervalSeconds > 0 {
		out.PollInterval = time.Duration(cfg.PollIntervalSeconds) * time.Second
	}
	return out
}

func StartReleaseIntentWorker(ctx context.Context, cfg ReleaseIntentWorkerConfig, processor ReleaseIntentProcessor) {
	if !cfg.Enabled || processor == nil {
		return
	}
	if cfg.WorkerID == "" {
		cfg.WorkerID = "release-worker"
	}
	if cfg.LeaseDuration <= 0 {
		cfg.LeaseDuration = time.Minute
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}
	go runReleaseIntentWorker(ctx, cfg, processor)
}

func runReleaseIntentWorker(ctx context.Context, cfg ReleaseIntentWorkerConfig, processor ReleaseIntentProcessor) {
	log := logger.LoggerWithContext(ctx)
	if log == nil {
		log = zap.NewNop()
	}
	log = log.With(
		zap.String("component", "release_intent_worker"),
		zap.String("worker_id", cfg.WorkerID),
		zap.String("execution_mode", string(GetExecutionMode())),
	)
	log.Info("release intent worker started",
		zap.String("result", "starting"),
		zap.Duration("lease_duration", cfg.LeaseDuration),
		zap.Duration("poll_interval", cfg.PollInterval),
	)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("release intent worker stopped", zap.String("result", "stopped"))
			return
		default:
		}

		processed, err := processor.ProcessNextReleaseIntent(ctx, cfg.WorkerID, cfg.LeaseDuration)
		if err != nil {
			log.Error("release intent worker iteration failed", zap.String("result", "error"), zap.Error(err))
		}
		if processed {
			continue
		}

		select {
		case <-ctx.Done():
			log.Info("release intent worker stopped", zap.String("result", "stopped"))
			return
		case <-ticker.C:
		}
	}
}
