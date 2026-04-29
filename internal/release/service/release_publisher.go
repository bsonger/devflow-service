package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"path"
	"strings"
	"time"

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
	archiveBytes, err := buildReleaseBundleArchive(req.Bundle)
	if err != nil {
		return nil, err
	}
	layer := content.NewDescriptorFromBytes(releaseBundleLayerMediaType, archiveBytes)
	layer.Annotations = map[string]string{
		ocispec.AnnotationTitle: releaseBundleArchiveName,
	}
	if err := store.Push(ctx, layer, bytes.NewReader(archiveBytes)); err != nil {
		return nil, err
	}
	manifestDesc, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_0, releaseBundleArtifactType, oras.PackManifestOptions{
		Layers: []ocispec.Descriptor{layer},
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
const releaseBundleArchiveName = "bundle.tar.gz"
const releaseBundleLayerMediaType = "application/vnd.oci.image.layer.v1.tar+gzip"

func buildReleaseBundleArchive(bundle *model.ReleaseBundle) ([]byte, error) {
	if bundle == nil {
		return nil, fmt.Errorf("release bundle is nil")
	}
	files := releaseBundleArtifactFiles(bundle)
	if len(files) == 0 {
		return nil, fmt.Errorf("release bundle has no files")
	}
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	gzipWriter.Header.ModTime = time.Unix(0, 0).UTC()
	gzipWriter.Header.Name = releaseBundleArchiveName
	gzipWriter.Header.Comment = ""
	tarWriter := tar.NewWriter(gzipWriter)
	for _, file := range files {
		filePath := normalizeReleaseBundleArchivePath(file.Path)
		if filePath == "" {
			continue
		}
		contentBytes := []byte(file.Content)
		header := &tar.Header{
			Name:     filePath,
			Mode:     0o644,
			Size:     int64(len(contentBytes)),
			ModTime:  time.Unix(0, 0).UTC(),
			Typeflag: tar.TypeReg,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, err
		}
		if _, err := tarWriter.Write(contentBytes); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, err
		}
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func releaseBundleArtifactFiles(bundle *model.ReleaseBundle) []model.ReleaseBundleFile {
	if bundle == nil {
		return nil
	}
	for _, file := range bundle.Files {
		if normalizeReleaseBundleArchivePath(file.Path) == "bundle.yaml" && strings.TrimSpace(file.Content) != "" {
			return []model.ReleaseBundleFile{{
				Path:    "bundle.yaml",
				Content: file.Content,
			}}
		}
	}
	combined := releaseBundleCombinedContent(bundle)
	if strings.TrimSpace(combined) == "" {
		return nil
	}
	return []model.ReleaseBundleFile{{
		Path:    "bundle.yaml",
		Content: combined,
	}}
}

func normalizeReleaseBundleArchivePath(filePath string) string {
	cleaned := path.Clean(strings.TrimSpace(filePath))
	cleaned = strings.TrimPrefix(cleaned, "/")
	switch cleaned {
	case "", ".", "..":
		return ""
	}
	if strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return cleaned
}

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
