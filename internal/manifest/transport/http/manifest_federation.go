package http

import (
	"context"

	"github.com/google/uuid"
)

func manifestShouldProxyByApplication(ctx context.Context, applicationID uuid.UUID) bool {
	_ = ctx
	_ = applicationID
	// Manifest is the build-side freeze point and stays on the control plane
	// that accepted the build request. Release ownership follows deploy target
	// ownership separately in release-service.
	return false
}
