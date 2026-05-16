package watch

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RunningRelease struct {
	ReleaseID      uuid.UUID
	ControlPlaneID string
	Status         string
}

type RunningReleaseLister interface {
	ListRunningReleases(ctx context.Context) ([]*RunningRelease, error)
}

type RunningReleaseSource struct {
	lister         RunningReleaseLister
	queue          ReleaseQueue
	controlPlaneID string
	pollInterval   time.Duration
	lastActive     map[string]struct{}
}

func NewRunningReleaseSource(lister RunningReleaseLister, queue ReleaseQueue, controlPlaneID string, pollInterval time.Duration) *RunningReleaseSource {
	if pollInterval <= 0 {
		pollInterval = 15 * time.Second
	}
	return &RunningReleaseSource{
		lister:         lister,
		queue:          queue,
		controlPlaneID: strings.TrimSpace(controlPlaneID),
		pollInterval:   pollInterval,
		lastActive:     map[string]struct{}{},
	}
}

func (s *RunningReleaseSource) Run(ctx context.Context) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	s.sync(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sync(ctx)
		}
	}
}

func (s *RunningReleaseSource) sync(ctx context.Context) {
	items, err := s.lister.ListRunningReleases(ctx)
	if err != nil {
		return
	}

	next := map[string]struct{}{}
	for _, item := range items {
		if item == nil {
			continue
		}
		if strings.TrimSpace(item.ControlPlaneID) != s.controlPlaneID {
			continue
		}
		if strings.ToLower(strings.TrimSpace(item.Status)) != "running" {
			continue
		}
		key := item.ReleaseID.String()
		next[key] = struct{}{}
		s.queue.Add(key)
	}

	for key := range s.lastActive {
		if _, ok := next[key]; !ok {
			s.queue.Add(key)
		}
	}
	s.lastActive = next
}
