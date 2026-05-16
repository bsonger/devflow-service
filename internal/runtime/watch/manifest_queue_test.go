package watch

import (
	"context"
	"testing"
	"time"
)

func TestManifestQueueRunsHandlerForAddedKey(t *testing.T) {
	queue := NewManifestQueue()
	defer queue.ShutDown()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan string, 1)
	go queue.Run(ctx, 1, func(_ context.Context, key string) error {
		done <- key
		cancel()
		return nil
	})

	queue.Add("manifest-1")

	select {
	case got := <-done:
		if got != "manifest-1" {
			t.Fatalf("key = %q, want manifest-1", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for manifest key")
	}
}
