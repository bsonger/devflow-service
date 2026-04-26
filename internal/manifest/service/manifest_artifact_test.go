package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	manifestdomain "github.com/bsonger/devflow-service/internal/manifest/domain"
	model "github.com/bsonger/devflow-service/internal/release/domain"
	"github.com/google/uuid"
)

type stubManifestArtifactPublisher struct {
	publishFn func(context.Context, manifestArtifactPackage) (*manifestArtifactPublishResult, error)
}

func (s stubManifestArtifactPublisher) Publish(ctx context.Context, pkg manifestArtifactPackage) (*manifestArtifactPublishResult, error) {
	return s.publishFn(ctx, pkg)
}

func TestPublishManifestArtifactPopulatesManifestFields(t *testing.T) {
	manifestID := mustUUID("11111111-1111-1111-1111-111111111111")
	pushedAt := time.Date(2026, time.April, 12, 11, 30, 0, 0, time.UTC)
	manifest := &manifestdomain.Manifest{
		BaseModel: model.BaseModel{
			ID:        manifestID,
			CreatedAt: time.Date(2026, time.April, 12, 11, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, time.April, 12, 11, 0, 0, 0, time.UTC),
		},
		ApplicationID: mustUUID("22222222-2222-2222-2222-222222222222"),
		EnvironmentID: "",
		ImageID:       mustUUID("33333333-3333-3333-3333-333333333333"),
		ImageRef:      "registry.example.com/devflow/demo-api@sha256:abc",
		RenderedYAML:  "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo-api\n",
		Status:        model.ManifestReady,
	}
	cfg := manifestdomain.ManifestRegistryConfig{
		Registry:   "registry.example.com",
		Namespace:  "devflow",
		Repository: "manifests",
	}
	var got manifestArtifactPackage
	err := publishManifestArtifact(context.Background(), manifest, "Demo API", cfg, stubManifestArtifactPublisher{
		publishFn: func(_ context.Context, pkg manifestArtifactPackage) (*manifestArtifactPublishResult, error) {
			got = pkg
			return &manifestArtifactPublishResult{
				Digest:    "sha256:def",
				MediaType: manifestOCIManifestMediaType,
				PushedAt:  pushedAt,
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Repository != "registry.example.com/devflow/manifests/demo-api" {
		t.Fatalf("repository = %q", got.Repository)
	}
	if got.Tag != "demo-api-20260412-110000" {
		t.Fatalf("tag = %q", got.Tag)
	}
	if got.Ref != got.Repository+":"+got.Tag {
		t.Fatalf("ref = %q", got.Ref)
	}
	if len(got.Layers) != 1 {
		t.Fatalf("layers = %d, want 1", len(got.Layers))
	}
	if got.Layers[0].Title != "bundle.tar.gz" {
		t.Fatalf("unexpected bundle layer %+v", got.Layers[0])
	}
	archiveFiles := readTarGzFiles(t, got.Layers[0].Bytes())
	if archiveFiles["manifest.yaml"] != manifest.RenderedYAML {
		t.Fatalf("manifest.yaml = %q", archiveFiles["manifest.yaml"])
	}
	if len(archiveFiles) != 1 {
		t.Fatalf("bundle files = %#v, want only manifest.yaml", archiveFiles)
	}
	var payload map[string]any
	if err := json.Unmarshal(got.Config, &payload); err != nil {
		t.Fatalf("unmarshal config json: %v", err)
	}
	if payload["image_ref"] != manifest.ImageRef {
		t.Fatalf("config image_ref = %#v", payload["image_ref"])
	}
	if manifest.ArtifactRef != got.Ref || manifest.ArtifactDigest != "sha256:def" {
		t.Fatalf("unexpected manifest artifact fields %+v", manifest)
	}
	if manifest.ArtifactMediaType != manifestOCIManifestMediaType {
		t.Fatalf("ArtifactMediaType = %q", manifest.ArtifactMediaType)
	}
	if manifest.ArtifactPushedAt == nil || !manifest.ArtifactPushedAt.Equal(pushedAt) {
		t.Fatalf("unexpected pushed at %v", manifest.ArtifactPushedAt)
	}
}

func readTarGzFiles(t *testing.T, archive []byte) map[string]string {
	t.Helper()

	gzr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	files := map[string]string{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read %s: %v", hdr.Name, err)
		}
		files[hdr.Name] = string(data)
	}
	return files
}

func TestPublishManifestArtifactPropagatesPublisherError(t *testing.T) {
	manifest := &manifestdomain.Manifest{
		BaseModel: model.BaseModel{
			ID:        uuid.New(),
			CreatedAt: time.Date(2026, time.April, 12, 11, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, time.April, 12, 11, 0, 0, 0, time.UTC),
		},
		ApplicationID: uuid.New(),
		EnvironmentID: "",
		ImageID:       uuid.New(),
		RenderedYAML:  "apiVersion: v1",
	}
	wantErr := errors.New("push failed")
	err := publishManifestArtifact(context.Background(), manifest, "demo-api", manifestdomain.ManifestRegistryConfig{
		Registry:   "registry.example.com",
		Namespace:  "devflow",
		Repository: "manifests",
	}, stubManifestArtifactPublisher{
		publishFn: func(_ context.Context, _ manifestArtifactPackage) (*manifestArtifactPublishResult, error) {
			return nil, wantErr
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("got err %v want %v", err, wantErr)
	}
	if manifest.ArtifactRef != "" || manifest.ArtifactDigest != "" {
		t.Fatalf("artifact fields should stay empty on failure: %+v", manifest)
	}
}
