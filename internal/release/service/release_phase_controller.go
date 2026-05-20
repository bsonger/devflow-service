package service

import (
	"context"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
)

type releasePhaseController struct {
	service *releaseService
}

func newReleasePhaseController(service *releaseService) *releasePhaseController {
	return &releasePhaseController{service: service}
}

func (c *releasePhaseController) runPrepareRelease(ctx context.Context, release *model.Release, manifest *manifestdomain.Manifest, app *releasesupport.ApplicationProjection, target releasesupport.DeployTarget) error {
	if err := c.service.renderDeploymentBundle(ctx, release, manifest, app, target); err != nil {
		return err
	}
	if err := c.service.publishDeploymentBundle(ctx, release, manifest, app, target); err != nil {
		return err
	}
	return nil
}

func (c *releasePhaseController) runHandoffDeployment(ctx context.Context, release *model.Release, manifest *manifestdomain.Manifest, app *releasesupport.ApplicationProjection, target releasesupport.DeployTarget) error {
	return c.service.createArgoApplication(ctx, release, manifest, app, target)
}

func (c *releasePhaseController) runObserveDeployment(ctx context.Context, release *model.Release, status model.ReleaseStatus) {
	c.service.markReleaseObservationTerminal(ctx, release, status)
}
