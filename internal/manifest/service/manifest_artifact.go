package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

const (
	manifestOCIArtifactType      = "application/vnd.devflow.manifest.bundle.v1"
	manifestOCIConfigMediaType   = "application/vnd.devflow.manifest.config.v1+json"
	manifestOCILayerMediaType    = "application/vnd.oci.image.layer.v1.tar+gzip"
	manifestOCIManifestMediaType = ocispec.MediaTypeImageManifest
)

type manifestArtifactLayer struct {
	Title     string
	MediaType string
	Content   string
}

func (l manifestArtifactLayer) Bytes() []byte {
	return []byte(l.Content)
}

type manifestArtifactPackage struct {
	Repository string
	Tag        string
	Ref        string
	Config     []byte
	Layers     []manifestArtifactLayer
}

type manifestArtifactPublishResult struct {
	Digest    string
	MediaType string
	PushedAt  time.Time
}

type manifestArtifactPublisher interface {
	Publish(context.Context, manifestArtifactPackage) (*manifestArtifactPublishResult, error)
}

type noopManifestArtifactPublisher struct{}

func (noopManifestArtifactPublisher) Publish(context.Context, manifestArtifactPackage) (*manifestArtifactPublishResult, error) {
	return nil, nil
}

type invalidManifestArtifactPublisher struct {
	err error
}

func (p invalidManifestArtifactPublisher) Publish(context.Context, manifestArtifactPackage) (*manifestArtifactPublishResult, error) {
	return nil, p.err
}

type orasManifestArtifactPublisher struct {
	cfg manifestdomain.ManifestRegistryConfig
}

func newManifestArtifactPublishing(cfg manifestdomain.ManifestRegistryConfig, enabled bool) manifestArtifactPublisher {
	if !enabled {
		return noopManifestArtifactPublisher{}
	}
	return orasManifestArtifactPublisher{cfg: cfg}
}

func publishManifestArtifact(ctx context.Context, manifest *manifestdomain.Manifest, applicationName string, cfg manifestdomain.ManifestRegistryConfig, publisher manifestArtifactPublisher) error {
	if publisher == nil {
		return nil
	}
	pkg, err := buildManifestArtifactPackage(manifest, applicationName, cfg)
	if err != nil {
		return err
	}
	result, err := publisher.Publish(ctx, pkg)
	if err != nil {
		return err
	}
	if result == nil {
		return nil
	}
	manifest.ArtifactRepository = pkg.Repository
	manifest.ArtifactTag = pkg.Tag
	manifest.ArtifactRef = pkg.Ref
	manifest.ArtifactDigest = result.Digest
	manifest.ArtifactMediaType = result.MediaType
	manifest.ArtifactPushedAt = &result.PushedAt
	return nil
}

func buildManifestArtifactPackage(manifest *manifestdomain.Manifest, applicationName string, cfg manifestdomain.ManifestRegistryConfig) (manifestArtifactPackage, error) {
	configJSON, err := json.Marshal(map[string]any{
		"manifest_id":    manifest.ID,
		"application_id": manifest.ApplicationID,
		"image_id":       manifest.ImageID,
		"image_ref":      manifest.ImageRef,
		"status":         manifest.Status,
		"created_at":     manifest.CreatedAt,
	})
	if err != nil {
		return manifestArtifactPackage{}, fmt.Errorf("marshal manifest artifact config: %w", err)
	}
	repository := cfg.RepositoryFor(applicationName, "")
	tag := manifestArtifactTag(applicationName, manifest.CreatedAt)
	bundle, err := buildManifestBundle(manifest.RenderedYAML)
	if err != nil {
		return manifestArtifactPackage{}, err
	}
	return manifestArtifactPackage{
		Repository: repository,
		Tag:        tag,
		Ref:        repository + ":" + tag,
		Config:     configJSON,
		Layers:     []manifestArtifactLayer{{Title: "bundle.tar.gz", MediaType: manifestOCILayerMediaType, Content: string(bundle)}},
	}, nil
}

func manifestArtifactTag(applicationName string, createdAt time.Time) string {
	name := strings.TrimSpace(applicationName)
	if name == "" {
		name = "manifest"
	}
	name = strings.ReplaceAll(strings.ToLower(name), "_", "-")
	name = strings.ReplaceAll(name, " ", "-")
	return name + "-" + createdAt.UTC().Format("20060102-150405")
}

func buildManifestBundle(renderedYAML string) ([]byte, error) {
	buf := new(bytes.Buffer)
	gzw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gzw)

	write := func(name string, data []byte) error {
		if err := tw.WriteHeader(&tar.Header{
			Name:    name,
			Mode:    0o644,
			Size:    int64(len(data)),
			ModTime: time.Now().UTC(),
		}); err != nil {
			return fmt.Errorf("write bundle header %s: %w", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			return fmt.Errorf("write bundle file %s: %w", name, err)
		}
		return nil
	}

	if err := write("manifest.yaml", []byte(renderedYAML)); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close bundle tar: %w", err)
	}
	if err := gzw.Close(); err != nil {
		return nil, fmt.Errorf("close bundle gzip: %w", err)
	}
	return buf.Bytes(), nil
}

func (p orasManifestArtifactPublisher) Publish(ctx context.Context, pkg manifestArtifactPackage) (*manifestArtifactPublishResult, error) {
	store := memory.New()

	configDesc := content.NewDescriptorFromBytes(manifestOCIConfigMediaType, pkg.Config)
	if err := store.Push(ctx, configDesc, bytes.NewReader(pkg.Config)); err != nil {
		return nil, fmt.Errorf("push manifest artifact config: %w", err)
	}
	layers := make([]ocispec.Descriptor, 0, len(pkg.Layers))
	for _, layer := range pkg.Layers {
		data := layer.Bytes()
		desc := content.NewDescriptorFromBytes(layer.MediaType, data)
		desc.Annotations = map[string]string{ocispec.AnnotationTitle: layer.Title}
		if err := store.Push(ctx, desc, bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("push manifest artifact layer %s: %w", layer.Title, err)
		}
		layers = append(layers, desc)
	}
	manifestDesc, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, manifestOCIArtifactType, oras.PackManifestOptions{
		Layers:           layers,
		ConfigDescriptor: &configDesc,
		ManifestAnnotations: map[string]string{
			ocispec.AnnotationCreated: time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("pack manifest artifact: %w", err)
	}
	if err := store.Tag(ctx, manifestDesc, pkg.Tag); err != nil {
		return nil, fmt.Errorf("tag manifest artifact: %w", err)
	}

	repo, err := remote.NewRepository(pkg.Repository)
	if err != nil {
		return nil, fmt.Errorf("build remote repository: %w", err)
	}
	repo.PlainHTTP = p.cfg.PlainHTTP
	if p.cfg.Username != "" || p.cfg.Password != "" {
		repo.Client = &auth.Client{
			Client: retry.DefaultClient,
			Cache:  auth.NewCache(),
			Credential: auth.StaticCredential(p.cfg.Registry, auth.Credential{
				Username: p.cfg.Username,
				Password: p.cfg.Password,
			}),
		}
	}
	pushedDesc, err := oras.Copy(ctx, store, pkg.Tag, repo, pkg.Tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("push manifest artifact: %w", err)
	}
	return &manifestArtifactPublishResult{
		Digest:    pushedDesc.Digest.String(),
		MediaType: pushedDesc.MediaType,
		PushedAt:  time.Now().UTC(),
	}, nil
}
