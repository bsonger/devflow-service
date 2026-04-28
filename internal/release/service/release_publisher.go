package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	releasesupport "github.com/bsonger/devflow-service/internal/release/support"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

type ReleaseBundlePublishRequest struct {
	Release        *model.Release
	Application    *releasesupport.ApplicationProjection
	Bundle         *model.ReleaseBundle
	RegistryConfig manifestdomain.ManifestRegistryConfig
}

type ReleaseBundlePublishResult struct {
	Repository string
	Tag        string
	Digest     string
	Ref        string
	Message    string
}

type releaseBundlePublisher interface {
	PublishBundle(ctx context.Context, req ReleaseBundlePublishRequest) (*ReleaseBundlePublishResult, error)
}

type metadataReleaseBundlePublisher struct{}
type orasReleaseBundlePublisher struct{}

var releaseBundlePublisherImpl releaseBundlePublisher = metadataReleaseBundlePublisher{}
var newOrasRemoteRepository = remote.NewRepository
var orasCopy = oras.Copy

func (metadataReleaseBundlePublisher) PublishBundle(_ context.Context, req ReleaseBundlePublishRequest) (*ReleaseBundlePublishResult, error) {
	repository, tag, digest, ref := deriveReleaseArtifactMetadataFromBundle(req.Release, req.Application, req.RegistryConfig, req.Bundle)
	message := "bundle published metadata recorded"
	if digest != "" {
		message = "bundle published metadata recorded with digest " + strings.TrimSpace(digest)
	}
	return &ReleaseBundlePublishResult{
		Repository: repository,
		Tag:        tag,
		Digest:     digest,
		Ref:        ref,
		Message:    message,
	}, nil
}

func (orasReleaseBundlePublisher) PublishBundle(ctx context.Context, req ReleaseBundlePublishRequest) (*ReleaseBundlePublishResult, error) {
	repository, tag, _, _ := deriveReleaseArtifactMetadataFromBundle(req.Release, req.Application, req.RegistryConfig, req.Bundle)
	store := memory.New()
	layers := make([]ocispec.Descriptor, 0, len(req.Bundle.Files))
	for _, file := range req.Bundle.Files {
		mediaType := releaseBundleFileMediaType(file.Path)
		desc := content.NewDescriptorFromBytes(mediaType, []byte(file.Content))
		if file.Path != "" {
			desc.Annotations = map[string]string{
				"org.opencontainers.image.title": file.Path,
			}
		}
		if err := store.Push(ctx, desc, bytes.NewReader([]byte(file.Content))); err != nil {
			return nil, err
		}
		layers = append(layers, desc)
	}
	manifestDesc, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, releaseBundleArtifactType, oras.PackManifestOptions{
		Layers: layers,
		ManifestAnnotations: map[string]string{
			"devflow.io/release-id": req.Release.ID.String(),
		},
	})
	if err != nil {
		return nil, err
	}
	digest := manifestDesc.Digest.String()
	if tag != "" {
		if err := store.Tag(ctx, manifestDesc, tag); err != nil {
			return nil, err
		}
	}
	ref := ""
	if repository != "" && digest != "" {
		ref = "oci://" + repository + "@" + digest
	}
	message := "bundle packaged with ORAS manifest metadata"
	if repository != "" && tag != "" {
		repo, err := buildOrasRemoteRepository(req.RegistryConfig, repository)
		if err != nil {
			return nil, err
		}
		copiedDesc, err := orasCopy(ctx, store, tag, repo, tag, oras.DefaultCopyOptions)
		if err != nil {
			return nil, err
		}
		digest = copiedDesc.Digest.String()
		if digest != "" {
			ref = "oci://" + repository + "@" + digest
		}
		message = fmt.Sprintf("bundle published to OCI via ORAS %s:%s", repository, tag)
	} else if digest != "" {
		message = "bundle packaged with ORAS manifest metadata " + strings.TrimSpace(digest)
	}
	return &ReleaseBundlePublishResult{
		Repository: repository,
		Tag:        tag,
		Digest:     digest,
		Ref:        ref,
		Message:    message,
	}, nil
}

const releaseBundleArtifactType = "application/vnd.devflow.release-bundle.v1"

func buildOrasRemoteRepository(cfg manifestdomain.ManifestRegistryConfig, repository string) (*remote.Repository, error) {
	repo, err := newOrasRemoteRepository(strings.TrimSpace(repository))
	if err != nil {
		return nil, err
	}
	repo.PlainHTTP = cfg.PlainHTTP
	username := strings.TrimSpace(cfg.Username)
	password := strings.TrimSpace(cfg.Password)
	if username == "" && password == "" {
		return repo, nil
	}
	repo.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(strings.TrimSpace(cfg.Registry), auth.Credential{
			Username: username,
			Password: password,
		}),
	}
	return repo, nil
}

func resolveReleaseBundlePublisher(runtimeCfg releasesupport.RuntimeConfig) releaseBundlePublisher {
	switch strings.ToLower(strings.TrimSpace(runtimeCfg.ManifestPublisherMode)) {
	case "oras":
		return orasReleaseBundlePublisher{}
	default:
		return releaseBundlePublisherImpl
	}
}

func releaseBundleFileMediaType(path string) string {
	lower := strings.ToLower(strings.TrimSpace(path))
	switch {
	case strings.HasSuffix(lower, ".yaml"), strings.HasSuffix(lower, ".yml"):
		return "application/yaml"
	default:
		return "application/octet-stream"
	}
}
