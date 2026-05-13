package http

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestManifestShouldNotProxyBuildSideManifest(t *testing.T) {
	if manifestShouldProxyByApplication(context.Background(), uuid.New()) {
		t.Fatal("build-side manifest creation must stay on the current control plane")
	}
}
