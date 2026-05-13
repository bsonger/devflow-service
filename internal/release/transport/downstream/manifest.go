package downstream

import (
	"context"
	"fmt"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	"github.com/bsonger/devflow-service/internal/shared/downstreamhttp"
)

type ManifestClient struct{ *downstreamhttp.Client }

func NewManifestClient(baseURL string) *ManifestClient {
	return &ManifestClient{Client: downstreamhttp.New(baseURL)}
}

func (c *ManifestClient) GetManifest(ctx context.Context, id string) (*manifestdomain.Manifest, error) {
	var out manifestdomain.Manifest
	if err := c.GetEnvelopeData(ctx, fmt.Sprintf("/api/v1/manifests/%s", id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
