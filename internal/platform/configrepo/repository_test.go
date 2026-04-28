package configrepo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type stubGitSyncer struct {
	commit string
	err    error
	calls  []string
}

func (s *stubGitSyncer) Sync(_ context.Context, rootDir, ref string) (string, error) {
	s.calls = append(s.calls, rootDir+"@"+ref)
	if s.err != nil {
		return "", s.err
	}
	return s.commit, nil
}

func TestRepositoryReadSnapshot(t *testing.T) {
	repo := NewRepository(Options{
		RootDir:    filepath.Join("testdata", "config-repo"),
		DefaultRef: "main",
	})

	snapshot, err := repo.ReadSnapshot(context.Background(), "devflow/devflow-app-service/staging", "")
	if err != nil {
		t.Fatalf("ReadSnapshot returned error: %v", err)
	}
	if snapshot.SourceCommit != "main" {
		t.Fatalf("SourceCommit = %q, want %q", snapshot.SourceCommit, "main")
	}
	if len(snapshot.Files) != 1 {
		t.Fatalf("len(Files) = %d, want 1", len(snapshot.Files))
	}
	if snapshot.Files[0].Name != "configuration.yaml" {
		t.Fatalf("first file = %q, want %q", snapshot.Files[0].Name, "configuration.yaml")
	}
	if snapshot.SourceDigest == "" {
		t.Fatal("SourceDigest should not be empty")
	}
}

func TestRepositoryReadSnapshotPullsGitRepoBeforeReading(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(rootDir, "devflow", "devflow-app-service", "staging")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "configuration.yaml"), []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	syncer := &stubGitSyncer{commit: "abc123def456"}
	repo := NewRepository(Options{
		RootDir:    rootDir,
		DefaultRef: "main",
	})
	repo.syncer = syncer

	snapshot, err := repo.ReadSnapshot(context.Background(), "devflow/devflow-app-service/staging", "")
	if err != nil {
		t.Fatalf("ReadSnapshot returned error: %v", err)
	}
	if len(syncer.calls) != 1 {
		t.Fatalf("sync calls = %d, want 1", len(syncer.calls))
	}
	if got := syncer.calls[0]; got != rootDir+"@main" {
		t.Fatalf("sync call = %q, want %q", got, rootDir+"@main")
	}
	if snapshot.SourceCommit != "abc123def456" {
		t.Fatalf("SourceCommit = %q, want %q", snapshot.SourceCommit, "abc123def456")
	}
}

func TestRepositoryReadSnapshotReturnsSyncError(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(rootDir, "devflow", "devflow-app-service", "staging")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "configuration.yaml"), []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(Options{
		RootDir:    rootDir,
		DefaultRef: "main",
	})
	repo.syncer = &stubGitSyncer{err: errors.New("pull failed")}

	_, err := repo.ReadSnapshot(context.Background(), "devflow/devflow-app-service/staging", "")
	if !errors.Is(err, ErrRepositorySyncFailed) {
		t.Fatalf("err = %v, want ErrRepositorySyncFailed", err)
	}
}

func TestRepositoryReadSnapshotFallsBackWhenGitSyncBinaryIsUnavailable(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(rootDir, "devflow", "devflow-app-service", "staging")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "configuration.yaml"), []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(Options{
		RootDir:    rootDir,
		DefaultRef: "main",
	})
	repo.syncer = &stubGitSyncer{err: errors.New("fork/exec /usr/bin/git: exec format error")}

	snapshot, err := repo.ReadSnapshot(context.Background(), "devflow/devflow-app-service/staging", "")
	if err != nil {
		t.Fatalf("ReadSnapshot returned error: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.SourceCommit != "main" {
		t.Fatalf("SourceCommit = %q, want %q", snapshot.SourceCommit, "main")
	}
	if len(snapshot.Files) != 1 {
		t.Fatalf("len(Files) = %d, want 1", len(snapshot.Files))
	}
}
